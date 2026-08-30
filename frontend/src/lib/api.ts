export interface BylawSection {
  id: string
  title: string
  content: string
  article: string
  domain?: string
}

export interface SearchResult {
  section: BylawSection
  score: number
  snippets: string[]
  matchType: 'title' | 'content' | 'both'
}

export interface SearchResponse {
  count: number
  query: string
  results: SearchResult[]
}

export interface ListResponse {
  count: number
  data: BylawSection[]
}

export async function searchBylaws(query: string): Promise<SearchResponse> {
  const res = await fetch(`/api/bylaws/search?q=${encodeURIComponent(query)}`)
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error || 'Search failed')
  }
  return res.json()
}

export async function listBylaws(): Promise<ListResponse> {
  const res = await fetch('/api/bylaws')
  if (!res.ok) {
    throw new Error('Failed to fetch bylaws')
  }
  return res.json()
}

export async function getBylaw(id: string): Promise<BylawSection> {
  const res = await fetch(`/api/bylaws/${encodeURIComponent(id)}`)
  if (!res.ok) {
    throw new Error('Bylaw section not found')
  }
  return res.json()
}

export interface ChatResponse {
  answer: string
  model: string
  sources?: string[]
}

export async function chatQuestion(question: string): Promise<ChatResponse> {
  const res = await fetch('/api/chat', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ question }),
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error || 'Chat request failed')
  }
  return res.json()
}

export async function healthCheck(): Promise<boolean> {
  try {
    const res = await fetch('/api/health')
    return res.ok
  } catch {
    return false
  }
}
