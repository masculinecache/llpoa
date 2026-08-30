package crawler

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/masculinecache/llpoa/search"
)

// Document represents a crawled document with metadata.
type Document struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Source    string    `json:"source"`    // e.g., "local", "gdrive", "legiscan"
	SourceURL string    `json:"sourceUrl"` // original URL or file path
	Category  string    `json:"category"`  // e.g., "bylaws", "restrictions", "meetings"
	CrawledAt time.Time `json:"crawledAt"`
}

// ContentSource is the interface for fetching documents from a source.
type ContentSource interface {
	Name() string
	Fetch() ([]Document, error)
}

// minStubLength is the minimum content length (in bytes) for a document to be
// indexed. Files shorter than this are table-of-contents stubs or empty
// placeholders that pollute search results without adding useful content.
const minStubLength = 200

func domainForDoc(doc Document) string {
	if doc.Source == "legiscan" {
		return "state"
	}
	if doc.Source == "local" && (strings.Contains(doc.SourceURL, "mcl-ch") || strings.HasPrefix(doc.Title, "mcl ch")) {
		return "state"
	}
	if doc.Source == "local" && strings.Contains(doc.SourceURL, "article-") {
		return "llpoa"
	}
	if doc.Source == "local" {
		return "county"
	}
	return "llpoa"
}

// isStub returns true if the document content is too short to be useful.
func isStub(doc Document) bool {
	return len(strings.TrimSpace(doc.Content)) < minStubLength
}

// ToBylawSections converts crawled documents into searchable bylaw sections.
// Stub filtering (content shorter than minStubLength) applies only to local
// files, where table-of-contents placeholders and empty files are common;
// remote sources such as LegiScan return only fully-formed documents.
// Documents with a stable ID keep it (legiscan bills, local files); documents
// without one are assigned a positional "doc-%d" ID.
func ToBylawSections(docs []Document) []search.BylawSection {
	var sections []search.BylawSection
	var stubCount int
	for i, doc := range docs {
		if doc.Source == "local" && isStub(doc) {
			stubCount++
			continue
		}
		article := ""
		if doc.Category != "" {
			article = doc.Category
		}
		id := doc.ID
		if id == "" {
			id = fmt.Sprintf("doc-%d", i)
		}
		sections = append(sections, search.BylawSection{
			ID:      id,
			Title:   doc.Title,
			Content: doc.Content,
			Article: article,
			Source:  doc.SourceURL,
			Domain:  domainForDoc(doc),
		})
	}
	if stubCount > 0 {
		log.Printf("Crawler: filtered %d stub documents (< %d bytes)", stubCount, minStubLength)
	}
	return sections
}
