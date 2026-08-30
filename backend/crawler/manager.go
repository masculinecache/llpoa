package crawler

import (
	"context"
	"log"
	"sync"

	"github.com/masculinecache/llpoa/search"
	"github.com/masculinecache/llpoa/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Manager orchestrates multiple content sources and indexes their output.
type Manager struct {
	mu      sync.RWMutex
	sources []ContentSource
	store   *search.Store
}

func NewManager(store *search.Store) *Manager {
	return &Manager{
		store: store,
	}
}

// AddSource registers a content source for crawling.
func (m *Manager) AddSource(source ContentSource) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sources = append(m.sources, source)
}

// CrawlAll fetches content from all registered sources and indexes it.
// Existing content (e.g., sample bylaws) is preserved and appended to.
func (m *Manager) CrawlAll(ctx context.Context) (int, error) {
	span, crawlCtx := tracing.StartSpan(ctx, "crawl.all")
	defer span.End()

	m.mu.RLock()
	sources := make([]ContentSource, len(m.sources))
	copy(sources, m.sources)
	m.mu.RUnlock()

	var allDocs []Document
	totalErrors := 0

	for _, source := range sources {
		sourceSpan, _ := tracing.StartSpan(crawlCtx, "crawl.source",
			trace.WithAttributes(attribute.String("source", source.Name())),
		)
		docs, err := source.Fetch()
		if err != nil {
			log.Printf("Crawler: source %q error: %v", source.Name(), err)
			sourceSpan.SetAttributes(attribute.Bool("error", true))
			sourceSpan.End()
			totalErrors++
			continue
		}
		sourceSpan.SetAttributes(attribute.Int("documents", len(docs)))
		sourceSpan.End()
		if len(docs) > 0 {
			log.Printf("Crawler: source %q fetched %d documents", source.Name(), len(docs))
			allDocs = append(allDocs, docs...)
		}
	}

	if len(allDocs) > 0 {
		sections := ToBylawSections(allDocs)

		// Get existing sections and append
		existing := m.store.List()
		combined := append(existing, sections...)
		m.store.Index(combined)

		log.Printf("Crawler: indexed %d new documents (total: %d)", len(sections), len(combined))
	} else {
		log.Printf("Crawler: no new documents found")
	}

	return len(allDocs), nil
}
