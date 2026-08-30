package search

import (
	"context"
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/masculinecache/llpoa/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// BylawSection represents a single section/article of the bylaws.
type BylawSection struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Article string `json:"article"`
	Source  string `json:"source,omitempty"`
	Domain  string `json:"domain,omitempty"`
}

// SearchResult contains a matching bylaw section with relevance info.
type SearchResult struct {
	Section   BylawSection `json:"section"`
	Score     float64      `json:"score"`
	Snippets  []string     `json:"snippets"`
	MatchType string       `json:"matchType"` // "title", "content", or "both"
}

// SearchOptions configures search behavior.
type SearchOptions struct {
	// Domains filters results to only these domains (empty = all domains).
	Domains []string
	// MaxResults caps the number of results returned (0 = no limit).
	MaxResults int
}

// Store holds and indexes bylaw sections for search.
type Store struct {
	mu       sync.RWMutex
	sections []BylawSection
	byID     map[string]int // index into sections slice
	bm25     *BM25          // BM25 index for scoring
}

// NewStore creates a new empty bylaw store.
func NewStore() *Store {
	return &Store{
		byID: make(map[string]int),
	}
}

// Index replaces all sections and rebuilds the in-memory index and BM25 stats.
func (s *Store) Index(sections []BylawSection) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sections = sections
	s.byID = make(map[string]int)
	for i, sec := range sections {
		s.byID[sec.ID] = i
	}

	// Build BM25 index from all document contents
	docs := make([]string, len(sections))
	for i, sec := range sections {
		docs[i] = sec.Title + " " + sec.Content
	}
	s.bm25 = NewBM25(docs)
}

// GetByID retrieves a single bylaw section by its ID.
func (s *Store) GetByID(id string) (BylawSection, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	idx, ok := s.byID[id]
	if !ok {
		return BylawSection{}, false
	}
	return s.sections[idx], true
}

// List returns all bylaw sections in order.
func (s *Store) List() []BylawSection {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]BylawSection, len(s.sections))
	copy(result, s.sections)
	return result
}

// Search performs a full-text search across all bylaw sections using BM25.
// Returns results ranked by relevance.
func (s *Store) Search(ctx context.Context, query string) []SearchResult {
	return s.SearchWithOptions(ctx, query, SearchOptions{})
}

// SearchWithOptions performs a full-text search with optional domain filtering
// and result limiting.
func (s *Store) SearchWithOptions(ctx context.Context, query string, opts SearchOptions) []SearchResult {
	span, _ := tracing.StartSpan(ctx, "search.fulltext",
		trace.WithAttributes(attribute.String("query", query)),
	)
	defer span.End()
	s.mu.RLock()
	defer s.mu.RUnlock()

	if strings.TrimSpace(query) == "" {
		return nil
	}

	queryLower := strings.ToLower(strings.TrimSpace(query))
	terms := tokenize(queryLower)

	if len(terms) == 0 {
		return nil
	}

	// Build domain filter set
	var domainSet map[string]bool
	if len(opts.Domains) > 0 {
		domainSet = make(map[string]bool, len(opts.Domains))
		for _, d := range opts.Domains {
			domainSet[strings.ToLower(d)] = true
		}
	}

	// Score with BM25
	bm25Results := s.bm25.ScoreAll(terms)

	var results []SearchResult
	for _, br := range bm25Results {
		section := s.sections[br.Idx]

		// Apply domain filter
		if domainSet != nil && section.Domain != "" && !domainSet[strings.ToLower(section.Domain)] {
			continue
		}

		// Determine match type by checking if query terms appear in title
		titleTerms := tokenize(strings.ToLower(section.Title))
		titleBM25 := 0.0
		for _, t := range terms {
			for _, tt := range titleTerms {
				if t == tt {
					titleBM25 += 1.0
					break
				}
			}
		}

		matchType := "content"
		if titleBM25 > 0 && br.Score > titleBM25 {
			matchType = "both"
		} else if titleBM25 > 0 {
			matchType = "title"
		}

		snippets := extractSnippets(section.Content, queryLower, 2)

		results = append(results, SearchResult{
			Section:   section,
			Score:     br.Score,
			Snippets:  snippets,
			MatchType: matchType,
		})
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Apply max results limit
	if opts.MaxResults > 0 && len(results) > opts.MaxResults {
		results = results[:opts.MaxResults]
	}

	return results
}

// extractSnippets returns up to maxSnippets context windows around matches.
func extractSnippets(content, query string, maxSnippets int) []string {
	lower := strings.ToLower(content)

	var snippets []string
	seen := make(map[string]bool)

	// Find the first occurrence
	idx := strings.Index(lower, query)
	if idx < 0 {
		return snippets
	}

	// Extract context windows around each match
	for i := 0; i < maxSnippets; i++ {
		if idx < 0 {
			break
		}

		// Get context: 80 chars before, the match, 80 chars after
		start := idx - 80
		if start < 0 {
			start = 0
		}
		end := idx + len(query) + 80
		if end > len(content) {
			end = len(content)
		}

		snippet := strings.TrimSpace(content[start:end])
		if !seen[snippet] {
			if start > 0 {
				snippet = "..." + snippet
			}
			if end < len(content) {
				snippet = snippet + "..."
			}
			snippets = append(snippets, snippet)
			seen[snippet] = true
		}

		// Find next occurrence
		if end < len(lower) {
			nextIdx := strings.Index(lower[end:], query)
			if nextIdx >= 0 {
				idx = end + nextIdx
			} else {
				idx = -1
			}
		} else {
			idx = -1
		}
	}

	return snippets
}

// Count returns the number of indexed sections.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sections)
}

// EstimateTokens returns a rough estimate of token count (chars / 4).
func EstimateTokens(text string) int {
	return int(math.Ceil(float64(len(text)) / 4.0))
}
