package main

import (
	"context"
	"embed"
	"encoding/json"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/masculinecache/llpoa/crawler"
	"github.com/masculinecache/llpoa/handlers"
	"github.com/masculinecache/llpoa/search"
	"github.com/masculinecache/llpoa/tracing"
)

//go:embed static/*
var staticFiles embed.FS

func main() {
	_ = godotenv.Load()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Initialize search store with sample bylaws
	bylawStore := search.NewStore()
	bylawStore.Index(loadSampleBylaws())

	// Initialize crawler manager and add sources
	crawlMgr := crawler.NewManager(bylawStore)

	// Local documents directory (user-placed .txt files)
	localDir := os.Getenv("DOCUMENTS_DIR")
	if localDir == "" {
		localDir = "documents"
	}
	crawlMgr.AddSource(crawler.NewLocalSource(localDir, "local"))

	// Michigan state legislative bills via the LegiScan API (only when a key
	// is configured). Bills land in the "state" domain alongside MCL chapters
	// in the same searchable index.
	if legiscanKey := os.Getenv("LEGISCAN_API_KEY"); legiscanKey != "" {
		legiscanSource := crawler.NewLegiScanSource(legiscanKey, os.Getenv("LEGISCAN_STATE"))
		if sessionID, err := strconv.Atoi(os.Getenv("LEGISCAN_SESSION_ID")); err == nil && sessionID > 0 {
			legiscanSource.SetSessionID(sessionID)
		}
		crawlMgr.AddSource(legiscanSource)
		log.Println("Crawler: LegiScan source enabled for Michigan state bills")
	}

	// Initialize OpenTelemetry tracing (Sentry + New Relic)
	tp, err := tracing.Init()
	if err != nil {
		log.Printf("WARN: tracing init error: %v (server continues without APM)", err)
	}
	if tp != nil {
		defer tracing.Shutdown(tp)
	}

	// Crawl sources synchronously so the local index is complete before
	// ListenAndServe — requests are never served from a partial index.
	ctx := context.Background()
	if n, err := crawlMgr.CrawlAll(ctx); err != nil {
		log.Printf("Crawl completed with errors: %v", err)
	} else if n > 0 {
		log.Printf("Crawled and indexed %d documents from all sources", n)
	} else {
		log.Println("No additional documents found via crawler — sample bylaws loaded. Place .txt files in documents/.")
	}

	// Set up handlers
	bylawHandler := handlers.NewBylawHandler(bylawStore)
	chatHandler := handlers.NewChatHandler(bylawStore)

	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("GET /api/bylaws", bylawHandler.ListBylaws)
	mux.HandleFunc("GET /api/bylaws/search", bylawHandler.SearchBylaws)
	mux.HandleFunc("GET /api/bylaws/{id}", bylawHandler.GetBylaw)
	mux.HandleFunc("POST /api/chat", chatHandler.Chat)
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"posthog_key": os.Getenv("POSTHOG_KEY"),
		})
	})

	// Serve static frontend
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("Failed to create static file server: %v", err)
	}
	fileServer := http.FileServer(http.FS(staticFS))

	// SPA fallback
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			f, err := staticFS.Open(strings.TrimPrefix(r.URL.Path, "/"))
			if err == nil {
				f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		data, err := staticFS.Open("index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer data.Close()
		stat, _ := data.Stat()
		http.ServeContent(w, r, "index.html", stat.ModTime(), data.(io.ReadSeeker))
	})

	addr := ":" + port
	log.Printf("LLPOA Bylaw Search server starting on %s", addr)

	handler := tracing.Middleware(mux)
	log.Printf("API: http://localhost%s/api/health", addr)
	log.Printf("UI:  http://localhost%s", addr)
	log.Printf("Chat: POST http://localhost%s/api/chat (requires OPENROUTER_API_KEY)", addr)

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func loadSampleBylaws() []search.BylawSection {
	return []search.BylawSection{
		{
			ID: "article-1", Title: "ARTICLE I - NAME AND OFFICE", Article: "I", Domain: "llpoa",
			Content: `Section 1. Name. The name of this Association shall be the Lake Louise Property Owners Association, hereafter referred to as the "Association."

Section 2. Principal Office. The principal office of the Association shall be located in Otsego County, Michigan. The Board of Directors may change the principal office from time to time.`,
		},
		{
			ID: "article-2", Title: "ARTICLE II - PURPOSE", Article: "II", Domain: "llpoa",
			Content: `Section 1. Purpose. The Association is organized exclusively for the benefit of the members of the Lake Louise Property Owners Association. The purpose of the Association shall be to promote the health, safety, and welfare of the members; to maintain and preserve the common areas and facilities; and to enforce the covenants, conditions, and restrictions applicable to the properties within the Association.

Section 2. Nonprofit Operation. The Association shall operate as a nonprofit organization under applicable Michigan law.`,
		},
		{
			ID: "article-3", Title: "ARTICLE III - MEMBERSHIP", Article: "III", Domain: "llpoa",
			Content: `Section 1. Members. Every owner of a lot or parcel within the Association's jurisdiction shall be a member of the Association. Membership shall be appurtenant to and may not be separated from ownership of the lot.

Section 2. Voting Rights. Each lot shall be entitled to one vote. If a lot is owned by more than one person, those owners shall agree on how to cast the single vote. No lot shall have more than one vote.

Section 3. Annual Meeting. The annual meeting of the members shall be held each year at a date and time determined by the Board of Directors. Notice of the annual meeting shall be given to each member at least fourteen (14) days prior to the meeting.

Section 4. Special Meetings. Special meetings of the members may be called by the President, a majority of the Board of Directors, or by members holding at least twenty-five percent (25%) of the total voting power.`,
		},
		{
			ID: "article-4", Title: "ARTICLE IV - BOARD OF DIRECTORS", Article: "IV", Domain: "llpoa",
			Content: `Section 1. Number and Term. The affairs of the Association shall be managed by a Board of Directors consisting of seven (7) members. Each director shall serve a term of three (3) years, with terms staggered so that approximately one-third of the directors are elected each year.

Section 2. Qualifications. Each director must be a member in good standing of the Association. No person may serve as a director if they are more than ninety (90) days delinquent in the payment of assessments.

Section 3. Election. Directors shall be elected by a plurality vote of the members at the annual meeting. Voting may be conducted in person or by proxy.

Section 4. Meetings. The Board shall meet at least quarterly. Special meetings may be called by the President or by any three directors upon at least three (3) days' notice.

Section 5. Compensation. Directors shall serve without compensation, but may be reimbursed for reasonable expenses incurred in the performance of their duties.`,
		},
		{
			ID: "article-5", Title: "ARTICLE V - OFFICERS", Article: "V", Domain: "llpoa",
			Content: `Section 1. Officers. The officers of the Association shall be a President, Vice President, Secretary, and Treasurer. All officers must be members of the Board of Directors.

Section 2. Election and Term. The Board of Directors shall elect the officers at its first meeting following the annual meeting of the members. Each officer shall serve a term of one (1) year.

Section 3. President. The President shall preside at all meetings of the members and the Board, shall appoint all committees, and shall perform all duties customary to the office.

Section 4. Vice President. The Vice President shall perform the duties of the President in the President's absence and such other duties as may be assigned.

Section 5. Secretary. The Secretary shall keep the minutes of all meetings, maintain the Association's records, and give all required notices.

Section 6. Treasurer. The Treasurer shall receive and deposit all funds of the Association, maintain accurate financial records, and prepare an annual budget for approval by the Board.`,
		},
		{
			ID: "article-6", Title: "ARTICLE VI - ASSESSMENTS", Article: "VI", Domain: "llpoa",
			Content: `Section 1. Annual Assessment. The Board of Directors shall establish an annual assessment to be paid by each member. The assessment shall be due and payable on January 1 of each year.

Section 2. Delinquency. Any assessment not paid within thirty (30) days of the due date shall be considered delinquent. A late fee of ten percent (10%) of the unpaid assessment shall be added.

Section 3. Lien Rights. The Association shall have a lien on each lot for unpaid assessments, which may be enforced in accordance with Michigan law.

Section 4. Use of Funds. Assessment funds shall be used exclusively for the operation, maintenance, and improvement of the Association's common areas and facilities, and for the administration of the Association.`,
		},
		{
			ID: "article-7", Title: "ARTICLE VII - COMMON AREAS", Article: "VII", Domain: "llpoa",
			Content: `Section 1. Common Areas. The Association shall maintain the common areas, including but not limited to roads, parks, boat launches, and recreational facilities, for the use and enjoyment of all members.

Section 2. Use Restrictions. No member shall use any common area in a manner that interferes with the rights of other members or damages the property. The Board may adopt reasonable rules and regulations governing the use of common areas.

Section 3. Lake Access. Lake access and riparian rights shall be maintained for the benefit of all members in accordance with the covenants and Michigan law.`,
		},
		{
			ID: "article-8", Title: "ARTICLE VIII - AMENDMENTS", Article: "VIII", Domain: "llpoa",
			Content: `Section 1. Amendments to Bylaws. These Bylaws may be amended by a majority vote of the members present at any annual or special meeting, provided that written notice of the proposed amendment has been given to all members at least fourteen (14) days prior to the meeting.

Section 2. Amendments to Covenants. Amendments to the Declaration of Covenants, Conditions, and Restrictions shall require the approval of at least two-thirds (2/3) of all members entitled to vote.

Section 3. Recording. All amendments shall be recorded with the Otsego County Register of Deeds.`,
		},
		{
			ID: "article-9", Title: "ARTICLE IX - INDEMNIFICATION", Article: "IX", Domain: "llpoa",
			Content: `Section 1. Indemnification. The Association shall indemnify every director, officer, and committee member against all expenses, judgments, fines, and amounts paid in settlement actually and reasonably incurred in connection with any legal proceeding arising out of their service to the Association, provided they acted in good faith and in a manner reasonably believed to be in the best interests of the Association.

Section 2. Insurance. The Association may purchase and maintain insurance on behalf of any person who is or was a director, officer, or committee member against any liability asserted against them in such capacity.`,
		},
		{
			ID: "article-10", Title: "ARTICLE X - MISCELLANEOUS", Article: "X", Domain: "llpoa",
			Content: `Section 1. Fiscal Year. The fiscal year of the Association shall begin on January 1 and end on December 31.

Section 2. Notices. Any notice required by these Bylaws shall be in writing and may be delivered personally, by mail, or by electronic means to the address of each member as it appears in the Association's records.

Section 3. Waiver of Notice. Waiver of any notice in writing signed by the person entitled to notice shall be equivalent to giving such notice.

Section 4. Severability. If any provision of these Bylaws is held invalid, the remainder of the Bylaws shall not be affected.

Section 5. Governing Law. These Bylaws shall be governed by and construed in accordance with the laws of the State of Michigan.`,
		},
	}
}
