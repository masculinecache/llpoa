package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/masculinecache/llpoa/search"
	"github.com/masculinecache/llpoa/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	// maxContextChars is the rough character budget for document context sent to the LLM.
	// ~2000 chars per result * 10 results = ~20K chars total context.
	maxContextCharsPerResult = 2000
	// maxResults is the number of search results to include in chat context.
	maxResults = 10
	// maxTotalContextChars is the total character budget for all document context.
	maxTotalContextChars = 20000
)

var systemPrompt = `You are an expert assistant for the Lake Louise Property Owners Association (LLPOA), specializing in LLPOA bylaws, Otsego County zoning ordinances, and Michigan Compiled Laws (MCL) related to property, zoning, planning, and corporate governance.

When answering:
1. Cite specific sections, articles, or statute numbers when referencing documents (e.g., "Article VI, Section 1" or "MCL 125.4228a").
2. Cross-reference between document domains when relevant — LLPOA bylaws may reference Michigan state law, and county zoning may interact with state planning requirements.
3. If a question involves multiple legal domains (e.g., assessment amounts + lien rights), synthesize information from all relevant sources.
4. If the answer is not in the provided documents, say "I don't have information about [topic] in the available documents" rather than guessing.
5. Be concise and precise — property owners need actionable answers, not legal dissertations.
6. When discussing dollar amounts, deadlines, or voting thresholds, quote the exact figures from the source.`

type ChatHandler struct {
	store  *search.Store
	apiKey string
}

func NewChatHandler(store *search.Store) *ChatHandler {
	return &ChatHandler{
		store:  store,
		apiKey: os.Getenv("OPENROUTER_API_KEY"),
	}
}

type ChatRequest struct {
	Question string `json:"question"`
}

type ChatResponse struct {
	Answer  string   `json:"answer"`
	Model   string   `json:"model"`
	Sources []string `json:"sources,omitempty"`
}

type ChatError struct {
	Error string `json:"error"`
}

func (h *ChatHandler) Chat(w http.ResponseWriter, r *http.Request) {
	if h.apiKey == "" {
		writeJSON(w, http.StatusServiceUnavailable, ChatError{
			Error: "OpenRouter API key not configured. Set OPENROUTER_API_KEY in environment.",
		})
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ChatError{Error: "invalid request body"})
		return
	}
	if strings.TrimSpace(req.Question) == "" {
		writeJSON(w, http.StatusBadRequest, ChatError{Error: "question is required"})
		return
	}

	// RAG: search for relevant content
	ragSpan, ragCtx := tracing.NewSpan(r, "rag.search")
	defer ragSpan.End()
	ragSpan.SetAttributes(attribute.String("question", req.Question))

	// Use domain-aware search with higher result count
	results := h.store.SearchWithOptions(ragCtx, req.Question, search.SearchOptions{
		MaxResults: maxResults * 2, // fetch extra for dedup/filtering
	})
	ragSpan.SetAttributes(attribute.Int("raw_result_count", len(results)))

	// Assemble context with token budget management
	var contextBuilder strings.Builder
	var sources []string
	contextBuilder.WriteString(systemPrompt)
	contextBuilder.WriteString("\n\n=== REFERENCE DOCUMENTS ===\n")

	totalChars := 0
	added := 0
	seenTitles := make(map[string]bool)

	for _, result := range results {
		if added >= maxResults {
			break
		}
		if totalChars >= maxTotalContextChars {
			break
		}

		section := result.Section

		// Deduplicate by title (skip near-identical MCL sections)
		if seenTitles[section.Title] {
			continue
		}
		seenTitles[section.Title] = true

		// Truncate content to budget per result
		content := section.Content
		if len(content) > maxContextCharsPerResult {
			content = content[:maxContextCharsPerResult] + "... [truncated]"
		}

		// Add domain metadata for LLM weighting
		domainLabel := ""
		switch section.Domain {
		case "llpoa":
			domainLabel = " [LLPOA Bylaws]"
		case "county":
			domainLabel = " [Otsego County Zoning]"
		case "state":
			domainLabel = " [Michigan Compiled Laws]"
		}

		sourceLabel := section.Title + domainLabel
		sources = append(sources, sourceLabel)

		contextBuilder.WriteString(fmt.Sprintf("\n--- Source: %s ---\n", sourceLabel))
		contextBuilder.WriteString(content)
		contextBuilder.WriteString("\n")

		totalChars += len(content) + len(sourceLabel) + 20
		added++
	}

	ragSpan.SetAttributes(attribute.Int("context_sources", added))
	ragSpan.SetAttributes(attribute.Int("context_chars", totalChars))

	// Call OpenRouter
	llmSpan, llmCtx := tracing.NewSpan(r, "rag.llm_call",
		trace.WithAttributes(attribute.Int("context_sources", added)),
	)
	defer llmSpan.End()

	prompt := contextBuilder.String() + fmt.Sprintf("\n=== USER QUESTION ===\n%s\n\nAnswer based on the reference documents above. Cite specific sections when possible.", req.Question)
	answer, model, err := callOpenRouter(llmCtx, h.apiKey, prompt)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ChatError{
			Error: fmt.Sprintf("AI service error: %v", err),
		})
		return
	}

	writeJSON(w, http.StatusOK, ChatResponse{
		Answer:  answer,
		Model:   model,
		Sources: sources,
	})
}

// Use OpenRouter's Free Models Router — automatically routes to the best free model.
var freeModels = []string{"openrouter/free"}

func callOpenRouter(ctx context.Context, apiKey, prompt string) (string, string, error) {
	body := map[string]interface{}{
		"model": freeModels[0],
		"messages": []map[string]interface{}{
			{"role": "user", "content": prompt},
		},
		"max_tokens":  1024,
		"temperature": 0.7,
	}

	var lastErr error
	for _, model := range freeModels {
		body["model"] = model
		payload, _ := json.Marshal(body)

		req, err := http.NewRequestWithContext(ctx, "POST", "https://openrouter.ai/api/v1/chat/completions", strings.NewReader(string(payload)))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("HTTP-Referer", "https://github.com/masculinecache/llpoa")
		req.Header.Set("X-Title", "LLPOA Bylaw Search")

		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		var result struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()

		if result.Error != nil {
			lastErr = fmt.Errorf("model %s: %s", model, result.Error.Message)
			continue
		}
		if len(result.Choices) > 0 && result.Choices[0].Message.Content != "" {
			return result.Choices[0].Message.Content, model, nil
		}
		lastErr = fmt.Errorf("model %s returned empty response", model)
	}

	return "", "", fmt.Errorf("all free models failed: %w", lastErr)
}
