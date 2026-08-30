package search

import (
	"math"
	"strings"
	"unicode"
)

const (
	defaultK1 = 1.2
	defaultB  = 0.75
)

// tokenize splits text into lowercase alphanumeric terms.
func tokenize(text string) []string {
	var result []string
	var buf strings.Builder
	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			buf.WriteRune(r)
		} else if buf.Len() > 0 {
			result = append(result, buf.String())
			buf.Reset()
		}
	}
	if buf.Len() > 0 {
		result = append(result, buf.String())
	}
	return result
}

// BM25 holds precomputed statistics and per-document term frequencies
// for BM25 scoring.
type BM25 struct {
	docCount  int
	avgDocLen float64
	k1        float64
	b         float64

	// df maps each term to the number of documents containing it.
	df map[string]int
	// tf[i] maps each term to its count in document i.
	tf []map[string]int
	// docLens[i] is the number of terms in document i.
	docLens []int
}

// NewBM25 builds a BM25 index from a set of documents.
func NewBM25(docs []string) *BM25 {
	b := &BM25{
		docCount: len(docs),
		df:       make(map[string]int),
		tf:       make([]map[string]int, len(docs)),
		docLens:  make([]int, len(docs)),
		k1:       defaultK1,
		b:        defaultB,
	}

	var totalLen int
	for i, doc := range docs {
		terms := tokenize(doc)
		b.docLens[i] = len(terms)
		totalLen += len(terms)

		tfMap := make(map[string]int)
		for _, term := range terms {
			tfMap[term]++
		}
		b.tf[i] = tfMap

		seen := make(map[string]bool)
		for term := range tfMap {
			if !seen[term] {
				b.df[term]++
				seen[term] = true
			}
		}
	}

	if b.docCount > 0 {
		b.avgDocLen = float64(totalLen) / float64(b.docCount)
	}

	return b
}

// Score computes the BM25 score for document docIdx against queryTerms.
func (b *BM25) Score(docIdx int, queryTerms []string) float64 {
	if b.docCount == 0 || b.avgDocLen == 0 {
		return 0
	}

	docLen := float64(b.docLens[docIdx])
	docTF := b.tf[docIdx]
	n := float64(b.docCount)

	var score float64
	for _, term := range queryTerms {
		tf := float64(docTF[term])
		if tf == 0 {
			continue
		}

		df := float64(b.df[term])

		// IDF: log((N - df + 0.5) / (df + 0.5) + 1)
		idf := math.Log((n-df+0.5)/(df+0.5) + 1.0)

		// TF with length normalization
		tfNorm := (tf * (b.k1 + 1)) / (tf + b.k1*(1-b.b+b.b*docLen/b.avgDocLen))

		score += idf * tfNorm
	}

	return score
}

// ScoreAll computes BM25 scores for all documents. Returns indices and scores
// for documents with score > 0, sorted by descending score.
type BM25Result struct {
	Idx   int
	Score float64
}

func (b *BM25) ScoreAll(queryTerms []string) []BM25Result {
	var results []BM25Result
	for i := 0; i < b.docCount; i++ {
		s := b.Score(i, queryTerms)
		if s > 0 {
			results = append(results, BM25Result{Idx: i, Score: s})
		}
	}
	return results
}
