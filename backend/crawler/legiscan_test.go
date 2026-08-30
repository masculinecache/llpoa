package crawler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newLegiScanTestSource spins up a canned LegiScan API server and returns a
// LegiScanSource pointed at it, plus the recorded ops it was called with.
func newLegiScanTestSource(t *testing.T, handler http.HandlerFunc) (*LegiScanSource, *[]string) {
	t.Helper()
	ops := &[]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*ops = append(*ops, r.URL.Query().Get("op"))
		handler(w, r)
	}))
	t.Cleanup(srv.Close)

	src := NewLegiScanSource("test-key", "MI")
	src.baseURL = srv.URL
	return src, ops
}

func testMasterListHandler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Query().Get("op") {
	case "getSessionList":
		fmt.Fprint(w, `{"status":"OK","sessions":[
			{"session_id":2183,"state_id":22,"state_abbr":"MI","sine_die":0,"session_title":"2025-2026 Regular Session"},
			{"session_id":2027,"state_id":22,"state_abbr":"MI","sine_die":1,"session_title":"2023-2024 Regular Session"}
		]}`)
	case "getMasterList":
		fmt.Fprint(w, `{"status":"OK","masterlist":{
			"session":{"session_id":2183,"session_title":"2025-2026 Regular Session"},
			"0":{"bill_id":1909445,"number":"HB4001","url":"https://legiscan.com/MI/bill/HB4001/2025","status":2,"status_date":"2025-01-23","last_action":"Read A First Time","last_action_date":"2025-01-09","title":"Labor: hours and wages; minimum hourly wage rate; modify.","description":"Labor: hours and wages; minimum hourly wage rate; modify."},
			"1":{"bill_id":1918428,"number":"HB4006","url":"https://legiscan.com/MI/bill/HB4006/2025","status":1,"status_date":"2025-01-14","last_action":"Bill Electronically Reproduced 01/14/2025","last_action_date":"2025-01-15","title":"Agriculture: agribusiness; exclusion of commercial weddings; prohibit.","description":"Agriculture: agribusiness: exclude commercial weddings."},
			"2":{"bill_id":3,"number":"","url":"","status":1,"title":""}
		}}`)
	default:
		http.Error(w, `{"status":"error","message":"bad op"}`, http.StatusBadRequest)
	}
}

// TestLegiScanFetchBuildsBills verifies the crawler converts the session's
// compact master list into searchable documents with stable IDs, readable
// status labels and the "state" domain.
func TestLegiScanFetchBuildsBills(t *testing.T) {
	src, ops := newLegiScanTestSource(t, testMasterListHandler)

	docs, err := src.Fetch()
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("Fetch returned %d docs, want 2 (empty-title bill skipped)", len(docs))
	}
	if docs[0].Title != "HB4001" || docs[0].ID != "legiscan-2183-HB4001" {
		t.Errorf("first bill = %q (%q), want HB4001 / legiscan-2183-HB4001", docs[0].Title, docs[0].ID)
	}
	if docs[0].Source != "legiscan" {
		t.Errorf("source = %q, want legiscan", docs[0].Source)
	}
	if !strings.Contains(docs[0].Content, "Status: Engrossed (2025-01-23)") {
		t.Errorf("content missing status line:\n%s", docs[0].Content)
	}
	if !strings.Contains(docs[0].Content, "https://legiscan.com/MI/bill/HB4001/2025") {
		t.Errorf("content missing bill URL:\n%s", docs[0].Content)
	}
	if !strings.Contains(docs[1].Content, "Agriculture: agribusiness: exclude commercial weddings.") {
		t.Errorf("description not included when it differs from title:\n%s", docs[1].Content)
	}

	sections := ToBylawSections(docs)
	if sections[0].Domain != "state" {
		t.Errorf("domain = %q, want state", sections[0].Domain)
	}
	if sections[0].ID != docs[0].ID {
		t.Errorf("section ID = %q, want stable doc ID %q", sections[0].ID, docs[0].ID)
	}

	// docs sorted by bill number
	if got := *ops; len(got) != 2 || got[0] != "getSessionList" || got[1] != "getMasterList" {
		t.Errorf("ops = %v, want [getSessionList getMasterList]", got)
	}
}

// TestLegiScanSessionPin verifies an explicit session id skips session
// discovery and targets only that session's master list.
func TestLegiScanSessionPin(t *testing.T) {
	var gotMaster []string
	src, ops := newLegiScanTestSource(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("op") == "getMasterList" {
			gotMaster = append(gotMaster, r.URL.Query().Get("id"))
		}
		fmt.Fprint(w, `{"status":"OK","masterlist":{"session":{"session_id":2027},"0":{"bill_id":1,"number":"SB0001","url":"https://legiscan.com/MI/bill/SB0001/2025","status":1,"status_date":"2025-01-14","title":"Test bill."}}}`)
	}))
	src.SetSessionID(2027)

	docs, err := src.Fetch()
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(*ops) != 1 || (*ops)[0] != "getMasterList" {
		t.Errorf("ops = %v, want only [getMasterList] when session pinned", *ops)
	}
	if len(gotMaster) != 1 || gotMaster[0] != "2027" {
		t.Errorf("getMasterList id(s) = %v, want [2027]", gotMaster)
	}
	if len(docs) != 1 || docs[0].ID != "legiscan-2027-SB0001" {
		t.Errorf("docs = %+v, want one legiscan-2027-SB0001", docs)
	}
}

// TestLegiScanAPIError verifies a non-OK upstream response surfaces as a
// source error (CrawlAll logs per-source errors and continues).
func TestLegiScanAPIError(t *testing.T) {
	src, _ := newLegiScanTestSource(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream down", http.StatusBadGateway)
	}))

	if _, err := src.Fetch(); err == nil {
		t.Fatal("Fetch succeeded, want error on upstream failure")
	}
}

// TestLegiScanStatusLabel covers the numeric status-code mapping.
func TestLegiScanStatusLabel(t *testing.T) {
	src := NewLegiScanSource("test-key", "MI")
	cases := map[int]string{
		1: "Introduced",
		2: "Engrossed",
		3: "Enrolled",
		4: "Passed",
		5: "Vetoed",
		6: "Failed",
		7: "Override",
		8: "Chaptered",
		9: "Status 9",
	}
	for code, want := range cases {
		if got := src.statusLabel(code); got != want {
			t.Errorf("statusLabel(%d) = %q, want %q", code, got, want)
		}
	}
}

// TestLegiScanStubsNotFiltered verifies legiscan bills are never dropped by
// the local-file stub filter even when their content is short.
func TestLegiScanStubsNotFiltered(t *testing.T) {
	docs := []Document{
		{ID: "legiscan-1-SB0001", Title: "SB0001", Content: "short", Source: "legiscan", SourceURL: "https://legiscan.com/MI/bill/SB0001/2025"},
		{ID: "local-a", Title: "table of contents", Content: "short", Source: "local", SourceURL: "/documents/toc.txt"},
	}
	sections := ToBylawSections(docs)
	if len(sections) != 1 {
		t.Fatalf("ToBylawSections returned %d sections, want 1 (stub filter is local-only)", len(sections))
	}
	if sections[0].ID != "legiscan-1-SB0001" || sections[0].Domain != "state" {
		t.Errorf("section = %+v, want legiscan-1-SB0001 with domain state", sections[0])
	}
}
