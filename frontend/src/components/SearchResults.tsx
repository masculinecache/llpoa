import type { SearchResult } from '../lib/api'

interface SearchResultsProps {
  results: SearchResult[]
  query: string
  onSelectSection: (id: string) => void
}

export function SearchResults({ results, query, onSelectSection }: SearchResultsProps) {
  if (results.length === 0) {
    return (
      <div className="search-results-empty">
        <p>No results found for "<strong>{query}</strong>". Try different keywords like "assessment", "meeting", "board", or "vote".</p>
      </div>
    )
  }

  return (
    <div className="search-results">
      <p className="search-results-heading">
        Showing {results.length} result{results.length !== 1 ? 's' : ''} for "<strong>{query}</strong>"
      </p>
      <ul>
        {results.map((result) => (
          <li key={result.section.id} className="search-result-item">
            <button
              className="search-result-link"
              onClick={() => onSelectSection(result.section.id)}
            >
              <div className="search-result-header">
                <h3>{result.section.title}</h3>
                <span className={`match-badge ${result.matchType}`}>
                  {result.matchType === 'both' ? 'Title & Content' : result.matchType === 'title' ? 'Title' : 'Content'}
                </span>
              </div>
              {result.snippets.length > 0 && (
                <div className="search-result-snippets">
                  {result.snippets.map((snippet, i) => (
                    <p key={i} className="snippet" dangerouslySetInnerHTML={{
                      __html: highlightMatch(snippet, query)
                    }} />
                  ))}
                </div>
              )}
              <div className="search-result-meta">
                <span className="relevance-score">
                  Relevance: {Math.round(result.score * 10)}%
                </span>
              </div>
            </button>
          </li>
        ))}
      </ul>
    </div>
  )
}

function highlightMatch(text: string, query: string): string {
  const escaped = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const regex = new RegExp(`(${escaped})`, 'gi')
  return text.replace(regex, '<mark class="highlight">$1</mark>')
}
