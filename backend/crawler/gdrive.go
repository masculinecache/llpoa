package crawler

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// GoogleDriveSource fetches documents from Google Drive links.
// Supports Google Docs (export as txt) and direct downloads for other file types.
type GoogleDriveSource struct {
	NameLabel string
	Category  string
	FileIDs   []string // Google Drive file IDs to fetch
	client    *http.Client
}

func NewGoogleDriveSource(name, category string, fileIDs []string) *GoogleDriveSource {
	return &GoogleDriveSource{
		NameLabel: name,
		Category:  category,
		FileIDs:   fileIDs,
		client:    &http.Client{Timeout: 8 * time.Second},
	}
}

// ExtractFileIDs parses Google Drive URLs and extracts the file IDs.
var driveFileRegex = regexp.MustCompile(`/file/d/([a-zA-Z0-9_-]+)`)

func ExtractFileIDs(urls []string) []string {
	var ids []string
	seen := make(map[string]bool)
	for _, url := range urls {
		matches := driveFileRegex.FindStringSubmatch(url)
		if len(matches) > 1 {
			id := matches[1]
			if !seen[id] {
				ids = append(ids, id)
				seen[id] = true
			}
		}
	}
	return ids
}

func (s *GoogleDriveSource) Name() string {
	return fmt.Sprintf("gdrive:%s", s.NameLabel)
}

func (s *GoogleDriveSource) Fetch() ([]Document, error) {
	if len(s.FileIDs) == 0 {
		return nil, nil
	}

	var docs []Document
	for i, fileID := range s.FileIDs {
		doc, err := s.fetchFile(fileID)
		if err != nil {
			// Log but continue with other files
			continue
		}
		if doc != nil {
			doc.Category = s.Category
			doc.ID = fmt.Sprintf("gdrive-%s-%d", s.Category, i)
			docs = append(docs, *doc)
		}
	}
	return docs, nil
}

func (s *GoogleDriveSource) fetchFile(fileID string) (*Document, error) {
	// First try: export as Google Doc (works for native Docs, Sheets, Slides)
	text, err := s.exportAsText(fileID)
	if err == nil && strings.TrimSpace(text) != "" {
		return &Document{
			Title:     fmt.Sprintf("Google Doc %s", fileID[:8]),
			Content:   text,
			Source:    "gdrive",
			SourceURL: fmt.Sprintf("https://drive.google.com/file/d/%s/view", fileID),
			CrawledAt: time.Now(),
		}, nil
	}

	// Second try: download as plain text via the download API
	text, err = s.downloadAsText(fileID)
	if err == nil && strings.TrimSpace(text) != "" {
		return &Document{
			Title:     fmt.Sprintf("Document %s", fileID[:8]),
			Content:   text,
			Source:    "gdrive",
			SourceURL: fmt.Sprintf("https://drive.google.com/file/d/%s/view", fileID),
			CrawledAt: time.Now(),
		}, nil
	}

	return nil, fmt.Errorf("could not fetch file %s", fileID)
}

func (s *GoogleDriveSource) exportAsText(fileID string) (string, error) {
	url := fmt.Sprintf("https://docs.google.com/document/d/%s/export?format=txt", fileID)
	return s.fetchURL(url)
}

func (s *GoogleDriveSource) downloadAsText(fileID string) (string, error) {
	// For PDFs and other binary formats, try direct download
	url := fmt.Sprintf("https://drive.google.com/uc?export=download&id=%s", fileID)
	resp, err := s.client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	text := string(body)
	// If content looks like HTML or is mostly binary, skip it
	if looksLikeHTML(text) || !isReadable(text) {
		return "", fmt.Errorf("file %s is not readable text", fileID)
	}

	return text, nil
}

func (s *GoogleDriveSource) fetchURL(url string) (string, error) {
	resp, err := s.client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func looksLikeHTML(content string) bool {
	trimmed := strings.TrimSpace(content)
	return strings.HasPrefix(trimmed, "<!") || strings.HasPrefix(trimmed, "<html")
}

func isReadable(content string) bool {
	if len(content) == 0 {
		return false
	}
	// Check ratio of printable chars to total
	printable := 0
	for _, r := range content {
		if r >= 32 && r <= 126 || r == '\n' || r == '\t' || r == '\r' {
			printable++
		}
	}
	return float64(printable)/float64(len(content)) > 0.7
}
