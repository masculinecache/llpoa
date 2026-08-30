import { useState, useEffect, useCallback } from 'react'
import { SearchBar } from './components/SearchBar'
import { BylawList } from './components/BylawList'
import { BylawViewer } from './components/BylawViewer'
import { SearchResults } from './components/SearchResults'
import { ChatSidebar } from './components/ChatSidebar'
import { searchBylaws, listBylaws, type BylawSection, type SearchResult } from './lib/api'

export default function App() {
  const [bylaws, setBylaws] = useState<BylawSection[]>([])
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [query, setQuery] = useState('')
  const [searchResults, setSearchResults] = useState<SearchResult[] | null>(null)
  const [isSearching, setIsSearching] = useState(false)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [sidebarOpen, setSidebarOpen] = useState(false)

  useEffect(() => {
    listBylaws()
      .then((res) => setBylaws(res.data))
      .catch((err) => setError(err.message))
      .finally(() => setIsLoading(false))
  }, [])

  const handleSearch = useCallback(async () => {
    if (!query.trim()) return
    setIsSearching(true)
    setError(null)
    try {
      const res = await searchBylaws(query)
      setSearchResults(res.results)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Search failed')
    } finally {
      setIsSearching(false)
    }
  }, [query])

  const handleSelectSection = useCallback((id: string) => {
    setSelectedId(id)
    setSearchResults(null)
    setQuery('')
    setSidebarOpen(false)
  }, [])

  const selectedSection = bylaws.find((b) => b.id === selectedId) ?? null
  const showLanding = !selectedSection && searchResults === null && !isLoading

  return (
    <div className="app">
      <header className="app-header">
        <div className="app-header-content">
          <button
            className="hamburger-button"
            onClick={() => setSidebarOpen(!sidebarOpen)}
            aria-label={sidebarOpen ? 'Close sidebar' : 'Open sidebar'}
          >
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              {sidebarOpen ? (
                <>
                  <path d="M18 6 6 18" />
                  <path d="m6 6 12 12" />
                </>
              ) : (
                <>
                  <line x1="3" y1="6" x2="21" y2="6" />
                  <line x1="3" y1="12" x2="21" y2="12" />
                  <line x1="3" y1="18" x2="21" y2="18" />
                </>
              )}
            </svg>
          </button>

          <div className="header-title-group">
            <img src="/icon.png" alt="LLPOA" className="header-logo" />
            <h1>LLPOA</h1>
            <span className="app-subtitle">Lake Louise POA</span>
          </div>

          <SearchBar
            query={query}
            onQueryChange={setQuery}
            onSearch={handleSearch}
            isSearching={isSearching}
            resultCount={searchResults !== null ? searchResults.length : null}
          />
        </div>
      </header>

      <div className="app-body">
        <div
          className={`app-sidebar-overlay ${sidebarOpen ? 'open' : ''}`}
          onClick={() => setSidebarOpen(false)}
        />

        <aside className={`app-sidebar ${sidebarOpen ? '' : 'closed'}`}>
          {isLoading ? (
            <div className="loading-state">
              <p>Loading...</p>
            </div>
          ) : (
            <BylawList
              bylaws={bylaws}
              selectedId={selectedId}
              onSelect={handleSelectSection}
            />
          )}
        </aside>

        <main className="app-content">
          {error && (
            <div className="error-banner">
              <p>{error}</p>
              <button onClick={() => setError(null)}>Dismiss</button>
            </div>
          )}

          {showLanding ? (
            <LandingPage
              llpoaCount={bylaws.filter(b => b.domain === 'llpoa').length}
              countyCount={bylaws.filter(b => b.domain === 'county').length}
              stateCount={bylaws.filter(b => b.domain === 'state').length}
            />
          ) : searchResults !== null ? (
            <SearchResults
              results={searchResults}
              query={query}
              onSelectSection={handleSelectSection}
            />
          ) : (
            <BylawViewer section={selectedSection} />
          )}
        </main>

        <aside className="app-chat-sidebar">
          <ChatSidebar onSelectSection={handleSelectSection} bylaws={bylaws} />
        </aside>
      </div>
    </div>
  )
}

function LandingPage({ llpoaCount, countyCount, stateCount }: {
  llpoaCount: number
  countyCount: number
  stateCount: number
}) {
  return (
    <div className="landing-page">
      <div className="landing-hero">
        <img src="/banner.jpeg" alt="Lake Louise POA" className="landing-banner" />
        <h2>LLPOA Document Portal</h2>
        <p className="landing-subtitle">
          A central reference for the Lake Louise Property Owners Association — browse governing bylaws,
          review county zoning restrictions, and look up relevant Michigan state law.
        </p>
      </div>

      <div className="landing-stats">
        <div className="landing-stat-card">
          <span className="landing-stat-number">{llpoaCount}</span>
          <span className="landing-stat-label">LLPOA Bylaws</span>
        </div>
        <div className="landing-stat-card">
          <span className="landing-stat-number">{countyCount}</span>
          <span className="landing-stat-label">Otsego County</span>
        </div>
        <div className="landing-stat-card">
          <span className="landing-stat-number">{stateCount}</span>
          <span className="landing-stat-label">Michigan State Law</span>
        </div>
      </div>

      <div className="landing-features">
        <div className="landing-feature-card">
          <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" className="landing-feature-icon">
            <line x1="8" y1="6" x2="21" y2="6" />
            <line x1="8" y1="12" x2="21" y2="12" />
            <line x1="8" y1="18" x2="21" y2="18" />
            <line x1="3" y1="6" x2="3.01" y2="6" />
            <line x1="3" y1="12" x2="3.01" y2="12" />
            <line x1="3" y1="18" x2="3.01" y2="18" />
          </svg>
          <div>
            <h4>Browse by Category</h4>
            <p>Articles grouped by domain — LLPOA Bylaws, County Restrictions, and State Law — with an expandable sidebar.</p>
          </div>
        </div>
        <div className="landing-feature-card">
          <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" className="landing-feature-icon">
            <circle cx="11" cy="11" r="8" />
            <path d="m21 21-4.3-4.3" />
          </svg>
          <div>
            <h4>Full-Text Search</h4>
            <p>Search across the entire document corpus instantly. Results show relevance scoring and context snippets.</p>
          </div>
        </div>
        <div className="landing-feature-card">
          <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" className="landing-feature-icon">
            <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
          </svg>
          <div>
            <h4>AI Chat</h4>
            <p>Ask questions in plain English. The assistant retrieves relevant documents and cites its sources.</p>
          </div>
        </div>
      </div>

      <div className="landing-tips">
        <h3>Quick Start</h3>
        <ul>
          <li>Open the <strong>sidebar</strong> (hamburger <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" style={{ display: 'inline', verticalAlign: 'middle' }}><line x1="3" y1="6" x2="21" y2="6" /><line x1="3" y1="12" x2="21" y2="12" /><line x1="3" y1="18" x2="21" y2="18" /></svg>) to browse documents by category</li>
          <li>Use the <strong>search bar</strong> at the top to find specific terms across all documents</li>
          <li>Click any search result or sidebar item to open the full document text</li>
          <li>Ask the <strong>AI Chat</strong> a question — it searches the documents and responds with citations</li>
        </ul>
      </div>
    </div>
  )
}
