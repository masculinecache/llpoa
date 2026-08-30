package crawler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LocalSource reads documents from a local directory.
// Supports .txt files (read directly) and attempts to read other common text formats.
type LocalSource struct {
	DirPath  string
	Category string
}

func NewLocalSource(dirPath, category string) *LocalSource {
	return &LocalSource{DirPath: dirPath, Category: category}
}

func (s *LocalSource) Name() string {
	return fmt.Sprintf("local:%s", s.DirPath)
}

func (s *LocalSource) Fetch() ([]Document, error) {
	entries, err := os.ReadDir(s.DirPath)
	if err != nil {
		// Directory doesn't exist yet — return empty, not an error
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading directory %s: %w", s.DirPath, err)
	}

	var docs []Document
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".txt" && ext != ".md" && ext != ".html" && ext != ".csv" {
			continue
		}

		fullPath := filepath.Join(s.DirPath, entry.Name())
		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		content := string(data)
		if strings.TrimSpace(content) == "" {
			continue
		}

		title := strings.TrimSuffix(entry.Name(), ext)
		// Use filename as title, clean up
		title = strings.ReplaceAll(title, "-", " ")
		title = strings.ReplaceAll(title, "_", " ")
		if len(title) > 100 {
			title = title[:100]
		}

		docs = append(docs, Document{
			ID:        fmt.Sprintf("local-%s-%d", s.Category, len(docs)),
			Title:     title,
			Content:   content,
			Source:    "local",
			SourceURL: fullPath,
			Category:  s.Category,
			CrawledAt: time.Now(),
		})
	}

	return docs, nil
}
