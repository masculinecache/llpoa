import type { BylawSection } from '../lib/api'

interface BylawViewerProps {
  section: BylawSection | null
}

export function BylawViewer({ section }: BylawViewerProps) {
  if (!section) {
    return (
      <div className="bylaw-viewer-empty">
        <div className="empty-state">
          <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" style={{ opacity: 0.3 }}>
            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
            <polyline points="14 2 14 8 20 8" />
            <line x1="16" y1="13" x2="8" y2="13" />
            <line x1="16" y1="17" x2="8" y2="17" />
            <polyline points="10 9 9 9 8 9" />
          </svg>
          <h3>Select a bylaw article</h3>
          <p>Choose an article from the sidebar or search for specific terms to get started.</p>
        </div>
      </div>
    )
  }

  return (
    <article className="bylaw-viewer">
      <header className="bylaw-viewer-header">
        <h2>{section.title}</h2>
        {section.article && <span className="bylaw-article-badge">Article {section.article}</span>}
      </header>
      <div className="bylaw-viewer-content">
        {section.content.split('\n').map((paragraph, i) => (
          paragraph.trim() ? <p key={i}>{paragraph}</p> : null
        ))}
      </div>
    </article>
  )
}
