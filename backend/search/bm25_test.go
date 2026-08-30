package search

import (
	"context"
	"os"
	"testing"

	"github.com/masculinecache/llpoa/tracing"
)

func TestMain(m *testing.M) {
	tracing.Init()
	os.Exit(m.Run())
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"Hello, World!", []string{"hello", "world"}},
		{"MCL 125.4228a: some text", []string{"mcl", "125", "4228a", "some", "text"}},
		{"  leading and trailing  ", []string{"leading", "and", "trailing"}},
		{"", nil},
		{"MiXeD CaSe TeSt", []string{"mixed", "case", "test"}},
		{"hyphenated-word another", []string{"hyphenated", "word", "another"}},
	}

	for _, tt := range tests {
		got := tokenize(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("tokenize(%q) = %v, want %v (len %d vs %d)", tt.input, got, tt.want, len(got), len(tt.want))
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("tokenize(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestBM25BasicScoring(t *testing.T) {
	docs := []string{
		"The assessment shall be due on January 1 of each year",
		"The board of directors meets quarterly to discuss zoning",
		"Lake access and boat launches are available to all members",
		"Assessment funds shall be used for maintenance and improvement",
	}
	bm25 := NewBM25(docs)

	terms := tokenize("assessment")
	scores := bm25.ScoreAll(terms)

	if len(scores) == 0 {
		t.Fatal("ScoreAll returned no results for 'assessment'")
	}

	// Docs 0 and 3 mention "assessment", docs 1 and 2 don't
	if scores[0].Idx != 0 && scores[0].Idx != 3 {
		t.Errorf("top result for 'assessment' is doc %d, want doc 0 or 3", scores[0].Idx)
	}

	// Verify non-matching docs have no score
	for _, s := range scores {
		if s.Idx == 1 || s.Idx == 2 {
			t.Errorf("non-matching doc %d scored %f, want 0", s.Idx, s.Score)
		}
	}
}

func TestBM25IDFRanking(t *testing.T) {
	// "zoning" appears in 1 of 4 docs, "the" appears in all
	docs := []string{
		"The zoning ordinance shall govern land use in all districts",
		"The assessment shall be paid annually by each member",
		"The board of directors meets quarterly under these bylaws",
		"The lake access rights are guaranteed to all property owners",
	}
	bm25 := NewBM25(docs)

	// "zoning" should rank doc 0 highest (unique term with high IDF)
	terms := tokenize("zoning")
	scores := bm25.ScoreAll(terms)
	if len(scores) == 0 || scores[0].Idx != 0 {
		t.Errorf("expected doc 0 to be top result for 'zoning', got %v", scores)
	}

	// "the" is in all docs — scores should be more uniform, doc 0 still wins
	// because it has the most terms overall
	terms = tokenize("the")
	scores = bm25.ScoreAll(terms)
	if len(scores) != 4 {
		t.Errorf("expected all 4 docs to match 'the', got %d", len(scores))
	}
}

func TestBM25LengthNormalization(t *testing.T) {
	// Short doc with "parking" once vs long doc with "parking" once
	// Short doc should score higher due to length normalization
	docs := []string{
		"Parking regulations apply here",
		"This is a very long document about many topics including parking and other municipal regulations that govern the use of property within the association boundaries and related matters",
	}
	bm25 := NewBM25(docs)

	terms := tokenize("parking")
	score0 := bm25.Score(0, terms)
	score1 := bm25.Score(1, terms)

	if score0 <= score1 {
		t.Errorf("short doc score %f should be > long doc score %f", score0, score1)
	}
}

func TestBM25MultipleTerms(t *testing.T) {
	docs := []string{
		"Parking restrictions apply to all vehicles in the association",
		"Vehicle storage must comply with the zoning ordinance requirements",
		"Parking and vehicle storage are regulated by this section of the bylaws",
	}
	bm25 := NewBM25(docs)

	// "parking" + "vehicle" — doc 0 is shorter with both terms, scores highest
	terms := tokenize("parking vehicle")
	scores := bm25.ScoreAll(terms)

	if len(scores) == 0 {
		t.Fatal("no results for 'parking vehicle'")
	}
	if scores[0].Idx != 0 {
		t.Errorf("expected doc 0 to top-score for 'parking vehicle', got doc %d", scores[0].Idx)
	}
}

func TestStoreSearchWithBM25(t *testing.T) {
	store := NewStore()
	store.Index([]BylawSection{
		{ID: "1", Title: "Assessment Amounts", Content: "Annual assessment shall not exceed $500 per lot per year.", Domain: "llpoa"},
		{ID: "2", Title: "Zoning Districts", Content: "The county establishes zoning districts including residential, commercial, and agricultural.", Domain: "county"},
		{ID: "3", Title: "MCL Zoning", Content: "The planning commission shall adopt a zoning ordinance.", Domain: "state"},
	})

	results := store.Search(context.Background(), "assessment")
	if len(results) == 0 {
		t.Fatal("no results for 'assessment'")
	}
	if results[0].Section.ID != "1" {
		t.Errorf("expected section 1 as top result, got %s", results[0].Section.ID)
	}
}

func TestStoreSearchDomainFilter(t *testing.T) {
	store := NewStore()
	store.Index([]BylawSection{
		{ID: "1", Title: "LLPOA Assessment", Content: "Annual assessment amounts for the association.", Domain: "llpoa"},
		{ID: "2", Title: "MCL Assessment", Content: "Assessment of property for tax purposes under state law.", Domain: "state"},
		{ID: "3", Title: "County Assessment", Content: "County assessment procedures and guidelines.", Domain: "county"},
	})

	// Search only LLPOA domain
	results := store.SearchWithOptions(context.Background(), "assessment", SearchOptions{
		Domains: []string{"llpoa"},
	})
	if len(results) != 1 {
		t.Fatalf("expected 1 LLPOA result, got %d", len(results))
	}
	if results[0].Section.Domain != "llpoa" {
		t.Errorf("expected llpoa domain, got %s", results[0].Section.Domain)
	}
}

func TestStoreSearchMaxResults(t *testing.T) {
	store := NewStore()
	sections := make([]BylawSection, 20)
	for i := range sections {
		sections[i] = BylawSection{
			ID:      string(rune('a' + i)),
			Title:   "Assessment Section",
			Content: "Assessment details for the property owners association.",
			Domain:  "llpoa",
		}
	}
	store.Index(sections)

	results := store.SearchWithOptions(context.Background(), "assessment", SearchOptions{
		MaxResults: 5,
	})
	if len(results) > 5 {
		t.Errorf("expected at most 5 results, got %d", len(results))
	}
}

func TestEstimateTokens(t *testing.T) {
	if got := EstimateTokens("hello"); got != 2 {
		t.Errorf("EstimateTokens('hello') = %d, want 2", got)
	}
	if got := EstimateTokens("12345678"); got != 2 {
		t.Errorf("EstimateTokens('12345678') = %d, want 2", got)
	}
	if got := EstimateTokens(""); got != 0 {
		t.Errorf("EstimateTokens('') = %d, want 0", got)
	}
}
