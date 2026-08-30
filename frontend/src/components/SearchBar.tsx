import { useRef } from 'react'

interface SearchBarProps {
  query: string
  onQueryChange: (query: string) => void
  onSearch: () => void
  isSearching: boolean
  resultCount: number | null
}

export function SearchBar({ query, onQueryChange, onSearch, isSearching, resultCount }: SearchBarProps) {
  const inputRef = useRef<HTMLInputElement>(null)

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    onSearch()
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'Escape') {
      onQueryChange('')
      inputRef.current?.focus()
    }
  }

  return (
    <form onSubmit={handleSubmit} className="search-form">
      <div className="search-input-wrapper">
        <svg className="search-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <circle cx="11" cy="11" r="8" />
          <path d="m21 21-4.35-4.35" />
        </svg>
        <input
          ref={inputRef}
          type="text"
          value={query}
          onChange={(e) => onQueryChange(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Search bylaws... (e.g., 'assessment', 'meeting', 'board')"
          className="search-input"
          autoFocus
        />
        {query && (
          <button type="button" onClick={() => { onQueryChange(''); inputRef.current?.focus() }} className="clear-button" aria-label="Clear search">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M18 6 6 18" /><path d="m6 6 12 12" />
            </svg>
          </button>
        )}
      </div>
      <button type="submit" className="search-button" disabled={isSearching || !query.trim()}>
        {isSearching ? 'Searching...' : 'Search'}
      </button>
      {resultCount !== null && !isSearching && (
        <span className="result-count">
          {resultCount} result{resultCount !== 1 ? 's' : ''}
        </span>
      )}
    </form>
  )
}
