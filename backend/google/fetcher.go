package google

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/masculinecache/llpoa/search"
)

// Fetcher retrieves bylaw content from a published Google Doc.
type Fetcher struct {
	client *http.Client
	apiKey string
}

// NewFetcher creates a new Google Doc fetcher.
// If apiKey is empty, only public/exported docs are accessible.
func NewFetcher(apiKey string) *Fetcher {
	return &Fetcher{
		client: &http.Client{Timeout: 30 * time.Second},
		apiKey: apiKey,
	}
}

// FetchBylaws fetches bylaw content from a Google Doc and parses it
// into structured sections. The doc must be published to the web or
// shared publicly. Uses the export to plain text format for reliability.
func (f *Fetcher) FetchBylaws(docID string) ([]search.BylawSection, error) {
	// Attempt to fetch as published plain text (no API key needed if doc is published to web)
	text, err := f.fetchPublishedDoc(docID)
	if err != nil {
		// Fall back to Google Docs API only if we have an API key
		if f.apiKey != "" {
			return f.fetchViaAPI(docID)
		}
		return nil, fmt.Errorf("cannot fetch doc: %w (publish the doc to the web or provide a GOOGLE_API_KEY)", err)
	}

	return parseBylawSections(text), nil
}

// fetchPublishedDoc fetches a Google Doc published to the web as plain text.
// URL format: https://docs.google.com/document/d/{DOCID}/export?format=txt
func (f *Fetcher) fetchPublishedDoc(docID string) (string, error) {
	url := fmt.Sprintf("https://docs.google.com/document/d/%s/export?format=txt", docID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch doc: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("doc fetch returned status %d (make sure the doc is published to the web)", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(body), nil
}

// fetchViaAPI uses the Google Docs API v1 to fetch structured content.
func (f *Fetcher) fetchViaAPI(docID string) ([]search.BylawSection, error) {
	url := fmt.Sprintf("https://docs.googleapis.com/v1/documents/%s?key=%s", docID, f.apiKey)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create API request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Docs API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Docs API returned status %d: %s", resp.StatusCode, string(body))
	}

	var docResponse struct {
		Title string `json:"title"`
		Body  struct {
			Content []struct {
				Paragraph struct {
					Elements []struct {
						TextRun struct {
							Content string `json:"content"`
						} `json:"textRun"`
					} `json:"elements"`
					ParagraphStyle struct {
						NamedStyleType string `json:"namedStyleType"`
					} `json:"paragraphStyle"`
				} `json:"paragraph"`
			} `json:"content"`
		} `json:"body"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&docResponse); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %w", err)
	}

	// Reconstruct plain text from the structured response
	var text strings.Builder
	for _, block := range docResponse.Body.Content {
		if block.Paragraph.Elements == nil {
			continue
		}
		for _, elem := range block.Paragraph.Elements {
			text.WriteString(elem.TextRun.Content)
		}
	}

	return parseBylawSections(text.String()), nil
}

// articleHeading matches common bylaw article headings like
// "ARTICLE I", "Article 1", "Article I - NAME", etc.
var articleHeading = regexp.MustCompile(`(?m)^\s*(ARTICLE\s+[IVXLCDM]+)\b`)

// sectionHeading matches "Section 1." or "Section 1." at the start of a line
var sectionHeading = regexp.MustCompile(`(?m)^\s*(Section\s+\d+\.?)\s*(.*)`)
var sectionHeadingAlt = regexp.MustCompile(`(?m)^\s*(\d+\.)\s+(.*)`)

// parseBylawSections parses raw text from a Google Doc into structured sections.
func parseBylawSections(text string) []search.BylawSection {
	lines := strings.Split(text, "\n")
	var sections []search.BylawSection

	var currentArticle string
	var currentTitle string
	var currentContent strings.Builder
	sectionCount := 0

	flushSection := func() {
		if currentContent.Len() > 0 {
			content := strings.TrimSpace(currentContent.String())
			if content != "" {
				id := fmt.Sprintf("section-%d", sectionCount)
				sections = append(sections, search.BylawSection{
					ID:      id,
					Title:   strings.TrimSpace(currentTitle),
					Content: content,
					Article: currentArticle,
				})
			}
		}
		currentContent.Reset()
		currentTitle = ""
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Check for article headings
		if matches := articleHeading.FindStringSubmatch(trimmed); len(matches) > 0 {
			flushSection()
			currentArticle = matches[1]
			currentTitle = trimmed
			sectionCount++
			continue
		}

		// Check for section headings
		if matches := sectionHeading.FindStringSubmatch(trimmed); len(matches) > 0 {
			flushSection()
			currentTitle = trimmed
			sectionCount++
			currentContent.WriteString(trimmed)
			currentContent.WriteString("\n")
			continue
		}

		// If we have a current section, append content
		if currentTitle != "" {
			currentContent.WriteString(trimmed)
			currentContent.WriteString(" ")
		}
	}

	flushSection()

	return sections
}
