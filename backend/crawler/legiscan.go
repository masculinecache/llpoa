package crawler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// LegiScanSource pulls state legislative bill metadata from the LegiScan.com
// JSON pull interface (https://api.legiscan.com/). It fetches the compact
// "master list" for a legislative session — every bill with its number, title,
// description, status and last action — in a single API request, so live bill
// tracking stays comfortably within the API key's request budget.
//
// Bills are converted into Documents and flow through the same crawler
// pipeline as the local corpus (see domainForDoc: legiscan -> "state"), making
// them searchable and browsable alongside MCL chapters and local documents.
type LegiScanSource struct {
	apiKey    string
	state     string
	sessionID int // 0 = auto-detect the current active session
	baseURL   string
	client    *http.Client
}

// NewLegiScanSource creates a source for the given state (defaults to "MI").
// SetSessionID can pin a specific legislative session; otherwise the most
// recent active session is discovered automatically on each Fetch.
func NewLegiScanSource(apiKey, state string) *LegiScanSource {
	if state == "" {
		state = "MI"
	}
	return &LegiScanSource{
		apiKey:  apiKey,
		state:   strings.ToUpper(state),
		baseURL: "https://api.legiscan.com",
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// SetSessionID pins the source to a specific legislative session id. A value
// of 0 (the default) auto-selects the current active session.
func (s *LegiScanSource) SetSessionID(sessionID int) {
	s.sessionID = sessionID
}

func (s *LegiScanSource) Name() string {
	return fmt.Sprintf("legiscan:%s", s.state)
}

// get issues a single LegiScan JSON pull request and decodes the response.
func (s *LegiScanSource) get(query string, out interface{}) error {
	url := fmt.Sprintf("%s/?key=%s&%s", s.baseURL, s.apiKey, query)
	resp, err := s.client.Get(url)
	if err != nil {
		return fmt.Errorf("legiscan: requesting %s: %w", query, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("legiscan: API returned status %d: %s", resp.StatusCode, string(body))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("legiscan: decoding response: %w", err)
	}
	return nil
}

type sessionEnvelope struct {
	Status   string `json:"status"`
	Sessions []struct {
		SessionID int    `json:"session_id"`
		SineDie   int    `json:"sine_die"`
		Title     string `json:"session_title"`
	} `json:"sessions"`
}

// resolveSessionID returns the session to crawl: the configured one when set,
// otherwise the first active (sine_die == 0) session, falling back to the most
// recent session returned by the API.
func (s *LegiScanSource) resolveSessionID() (int, error) {
	if s.sessionID != 0 {
		return s.sessionID, nil
	}
	var env sessionEnvelope
	if err := s.get(fmt.Sprintf("op=getSessionList&state=%s", s.state), &env); err != nil {
		return 0, err
	}
	if env.Status != "OK" || len(env.Sessions) == 0 {
		return 0, fmt.Errorf("legiscan: no sessions returned for state %s", s.state)
	}
	for _, sess := range env.Sessions {
		if sess.SineDie == 0 {
			return sess.SessionID, nil
		}
	}
	return env.Sessions[0].SessionID, nil
}

type masterListEnvelope struct {
	Status     string                     `json:"status"`
	MasterList map[string]json.RawMessage `json:"masterlist"`
}

// billEntry is the compact bill record returned by getMasterList.
type billEntry struct {
	BillID         int    `json:"bill_id"`
	Number         string `json:"number"`
	URL            string `json:"url"`
	Status         int    `json:"status"`
	StatusDate     string `json:"status_date"`
	LastAction     string `json:"last_action"`
	LastActionDate string `json:"last_action_date"`
	Title          string `json:"title"`
	Description    string `json:"description"`
}

// statusLabels maps LegiScan's numeric bill status codes to readable labels.
var statusLabels = map[int]string{
	1: "Introduced",
	2: "Engrossed",
	3: "Enrolled",
	4: "Passed",
	5: "Vetoed",
	6: "Failed",
	7: "Override",
	8: "Chaptered",
}

// statusLabel returns a readable label for a LegiScan status code.
func (s *LegiScanSource) statusLabel(code int) string {
	if label, ok := statusLabels[code]; ok {
		return label
	}
	return fmt.Sprintf("Status %d", code)
}

// Fetch retrieves metadata for every bill in the target session and converts
// it into searchable documents. The session's compact master list is fetched
// in a single API call.
func (s *LegiScanSource) Fetch() ([]Document, error) {
	sessionID, err := s.resolveSessionID()
	if err != nil {
		return nil, err
	}

	var env masterListEnvelope
	if err := s.get(fmt.Sprintf("op=getMasterList&id=%d", sessionID), &env); err != nil {
		return nil, err
	}
	if env.Status != "OK" {
		return nil, fmt.Errorf("legiscan: master list request failed (status %q)", env.Status)
	}

	var docs []Document
	for key, raw := range env.MasterList {
		if key == "session" {
			continue
		}
		var bill billEntry
		if err := json.Unmarshal(raw, &bill); err != nil {
			continue
		}
		if bill.Number == "" || strings.TrimSpace(bill.Title) == "" {
			continue
		}
		docs = append(docs, s.billToDocument(sessionID, bill))
	}

	// sort by bill number (zero-padded, so lexicographic order is numeric)
	sort.Slice(docs, func(i, j int) bool { return docs[i].Title < docs[j].Title })
	return docs, nil
}

// billToDocument renders a bill as a searchable document. The descriptive
// title, status and last action live in Content so snippets and BM25 match
// subject terms; Title is the bill number (e.g. "HB4006") so the sidebar and
// result lists stay scannable.
func (s *LegiScanSource) billToDocument(sessionID int, bill billEntry) Document {
	var content strings.Builder
	content.WriteString(strings.TrimSpace(bill.Title))
	if desc := strings.TrimSpace(bill.Description); desc != "" && desc != strings.TrimSpace(bill.Title) {
		content.WriteString("\n\n")
		content.WriteString(desc)
	}
	content.WriteString("\n\nStatus: ")
	content.WriteString(s.statusLabel(bill.Status))
	if bill.StatusDate != "" {
		content.WriteString(" (")
		content.WriteString(bill.StatusDate)
		content.WriteString(")")
	}
	if bill.LastAction != "" {
		content.WriteString("\nLast action: ")
		content.WriteString(bill.LastAction)
		if bill.LastActionDate != "" {
			content.WriteString(" (")
			content.WriteString(bill.LastActionDate)
			content.WriteString(")")
		}
	}
	if bill.URL != "" {
		content.WriteString("\n\nLegiScan: ")
		content.WriteString(bill.URL)
	}
	return Document{
		ID:        fmt.Sprintf("legiscan-%d-%s", sessionID, bill.Number),
		Title:     bill.Number,
		Content:   content.String(),
		Source:    "legiscan",
		SourceURL: bill.URL,
		CrawledAt: time.Now(),
	}
}
