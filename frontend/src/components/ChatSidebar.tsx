import { useState, useRef, useEffect } from 'react'
import type { BylawSection } from '../lib/api'
import { trackChat } from '../lib/posthog'

interface ChatMessage {
  role: 'user' | 'assistant'
  content: string
  model?: string
  sources?: string[]
}

interface ChatResponse {
  answer: string
  model: string
  sources?: string[]
}

interface ChatSidebarProps {
  onSelectSection: (id: string) => void
  bylaws: BylawSection[]
}

export function ChatSidebar({ onSelectSection, bylaws }: ChatSidebarProps) {
  const [messages, setMessages] = useState<ChatMessage[]>([{
    role: 'assistant',
    content: "Hi! Ask me anything about the bylaws, Otsego County zoning, or Michigan state law.",
  }])
  const [input, setInput] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [apiKeyMissing, setApiKeyMissing] = useState(false)
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  useEffect(() => {
    // Silently check if backend has the API key
    fetch('/api/health')
      .then(r => r.json())
      .catch(() => {})
  }, [])

  function findArticleId(title: string): string | null {
    for (const b of bylaws) {
      if (b.title === title || b.title.replace(/\.txt$/i, '') === title) {
        return b.id
      }
    }
    // Fuzzy match: check if source title is contained in or matches a bylaw title
    const normalized = title.toLowerCase().replace(/\.txt$/i, '').trim()
    for (const b of bylaws) {
      const bTitle = b.title.toLowerCase().replace(/\.txt$/i, '').trim()
      if (bTitle === normalized || bTitle.includes(normalized) || normalized.includes(bTitle)) {
        return b.id
      }
    }
    return null
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!input.trim() || isLoading) return

    const question = input.trim()
    setInput('')
    setError(null)
    setApiKeyMissing(false)
    setMessages(prev => [...prev, { role: 'user', content: question }])
    setIsLoading(true)
    const startTime = performance.now()

    try {
      const res = await fetch('/api/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ question }),
      })

      if (res.status === 503) {
        setApiKeyMissing(true)
        throw new Error('OpenRouter API key not configured')
      }

      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }))
        throw new Error(err.error || 'Chat request failed')
      }

      const data: ChatResponse = await res.json()
      const latencyMs = Math.round(performance.now() - startTime)
      trackChat(question, { model: data.model, sources: data.sources }, latencyMs)
      setMessages(prev => [...prev, {
        role: 'assistant',
        content: data.answer,
        model: data.model,
        sources: data.sources,
      }])
    } catch (err) {
      const latencyMs = Math.round(performance.now() - startTime)
      trackChat(question, {}, latencyMs, err instanceof Error ? err.message : 'Failed to get answer')
      setError(err instanceof Error ? err.message : 'Failed to get answer')
    } finally {
      setIsLoading(false)
    }
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSubmit(e)
    }
  }

  function handleSourceClick(title: string) {
    const id = findArticleId(title)
    if (id) {
      onSelectSection(id)
    }
  }

  return (
    <div className="chat-sidebar">
      <div className="chat-sidebar-header">
        <h3>AI Chat</h3>
      </div>
      <div className="chat-sidebar-messages">
        {messages.map((msg, i) => (
          <div key={i} className={`chat-sidebar-message ${msg.role}`}>
            <div className="chat-sidebar-avatar">
              {msg.role === 'assistant' ? (
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M12 2a4 4 0 0 1 4 4v2a4 4 0 0 1-8 0V6a4 4 0 0 1 4-4Z" />
                  <path d="M16 14H8a4 4 0 0 0-4 4v2h16v-2a4 4 0 0 0-4-4Z" />
                </svg>
              ) : (
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <circle cx="12" cy="8" r="4" />
                  <path d="M20 21a8 8 0 0 0-16 0" />
                </svg>
              )}
            </div>
            <div className="chat-sidebar-bubble">
              <p className="chat-sidebar-text">{msg.content}</p>
              {msg.model && (
                <p className="chat-sidebar-model">via {msg.model}</p>
              )}
              {msg.sources && msg.sources.length > 0 && (
                <details className="chat-sidebar-sources">
                  <summary>Sources ({msg.sources.length})</summary>
                  <ul>
                    {msg.sources.map((src, j) => (
                      <li key={j}>
                        <button
                          className="chat-source-link"
                          onClick={() => handleSourceClick(src)}
                          title="Open this document"
                        >
                          {src}
                        </button>
                      </li>
                    ))}
                  </ul>
                </details>
              )}
            </div>
          </div>
        ))}

        {isLoading && (
          <div className="chat-sidebar-message assistant">
            <div className="chat-sidebar-avatar">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M12 2a4 4 0 0 1 4 4v2a4 4 0 0 1-8 0V6a4 4 0 0 1 4-4Z" />
                <path d="M16 14H8a4 4 0 0 0-4 4v2h16v-2a4 4 0 0 0-4-4Z" />
              </svg>
            </div>
            <div className="chat-sidebar-bubble">
              <p className="chat-sidebar-thinking">
                <span className="dot">.</span>
                <span className="dot">.</span>
                <span className="dot">.</span>
              </p>
            </div>
          </div>
        )}

        {error && (
          <div className="chat-sidebar-error">
            <p>{error}</p>
          </div>
        )}

        <div ref={messagesEndRef} />
      </div>

      <form onSubmit={handleSubmit} className="chat-sidebar-input-area">
        <input
          ref={inputRef}
          type="text"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Ask a question..."
          disabled={isLoading}
          className="chat-sidebar-input"
        />
        <button
          type="submit"
          className="chat-sidebar-send"
          disabled={isLoading || !input.trim()}
        >
          {isLoading ? (
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="spinner">
              <path d="M21 12a9 9 0 1 1-6.219-8.56" />
            </svg>
          ) : (
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="m22 2-7 20-4-9-9-4Z" />
            </svg>
          )}
        </button>
      </form>

      {apiKeyMissing && (
        <p className="chat-sidebar-warning">
          AI requires an OpenRouter API key. Set <code>OPENROUTER_API_KEY</code>.
        </p>
      )}
    </div>
  )
}
