package crawler

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/masculinecache/llpoa/search"
	"github.com/masculinecache/llpoa/tracing"
)

func TestMain(m *testing.M) {
	tracing.Init()
	os.Exit(m.Run())
}

func sampleBylaws(n int) []search.BylawSection {
	sections := make([]search.BylawSection, n)
	for i := range sections {
		sections[i] = search.BylawSection{
			ID:      "sample-" + string(rune('A'+i)),
			Title:   "Sample Bylaw",
			Content: "Sample bylaw content for testing.",
		}
	}
	return sections
}

// TestColdStartIndexesLocalDocsSynchronously mirrors the startup sequence in
// main.go: samples are indexed first, then CrawlAll runs synchronously over
// the local source so every document is searchable before the server starts.
func TestColdStartIndexesLocalDocsSynchronously(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"doc-one.txt", "doc-two.txt", "doc-three.txt"} {
		content := "Restrictions for " + name + ": This document establishes the rules and regulations governing the use and enjoyment of the property within the association. No Zanzibar flamingo permits are allowed under any circumstances without prior written approval from the board of directors."
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("writing fixture: %v", err)
		}
	}

	store := search.NewStore()
	store.Index(sampleBylaws(10))

	mgr := NewManager(store)
	mgr.AddSource(NewLocalSource(dir, "local"))

	n, err := mgr.CrawlAll(context.Background())
	if err != nil {
		t.Fatalf("CrawlAll: %v", err)
	}
	if n != 3 {
		t.Errorf("CrawlAll fetched %d docs, want 3", n)
	}
	if got := store.Count(); got != 13 {
		t.Errorf("store has %d sections after cold start, want 13", got)
	}
	results := store.Search(context.Background(), "Zanzibar")
	if len(results) == 0 {
		t.Error("local document content is not searchable immediately after synchronous crawl")
	}
}

// TestColdStartWithMissingDocumentsDir ensures a missing documents directory
// never blocks startup or evicts the sample index.
func TestColdStartWithMissingDocumentsDir(t *testing.T) {
	store := search.NewStore()
	store.Index(sampleBylaws(10))

	mgr := NewManager(store)
	mgr.AddSource(NewLocalSource(filepath.Join(t.TempDir(), "does-not-exist"), "local"))

	if _, err := mgr.CrawlAll(context.Background()); err != nil {
		t.Fatalf("CrawlAll with missing dir should not error: %v", err)
	}
	if got := store.Count(); got != 10 {
		t.Errorf("store has %d sections, want 10 samples preserved", got)
	}
}
