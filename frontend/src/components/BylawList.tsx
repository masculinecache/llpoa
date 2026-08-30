import { useState, useMemo, useRef, useEffect } from 'react'
import type { BylawSection } from '../lib/api'

interface BylawListProps {
  bylaws: BylawSection[]
  selectedId: string | null
  onSelect: (id: string) => void
}

interface DomainGroup {
  id: string
  label: string
  sections: BylawSection[]
  defaultOpen: boolean
}

function buildGroups(bylaws: BylawSection[]): DomainGroup[] {
  const groups = new Map<string, BylawSection[]>()
  for (const b of bylaws) {
    const domain = b.domain || 'other'
    if (!groups.has(domain)) groups.set(domain, [])
    groups.get(domain)!.push(b)
  }

  const result: DomainGroup[] = []
  const llpoa = groups.get('llpoa')
  if (llpoa) {
    result.push({ id: 'llpoa', label: 'LLPOA Bylaws', sections: llpoa, defaultOpen: true })
  }
  const county = groups.get('county')
  if (county) {
    result.push({ id: 'county', label: 'Otsego County', sections: county, defaultOpen: false })
  }
  const stateGroup = groups.get('state')
  if (stateGroup) {
    result.push({ id: 'state', label: 'Michigan State Law', sections: stateGroup, defaultOpen: false })
  }
  for (const [key, sections] of groups) {
    if (key !== 'llpoa' && key !== 'county' && key !== 'state') {
      result.push({ id: key, label: key, sections, defaultOpen: false })
    }
  }
  return result
}

export function BylawList({ bylaws, selectedId, onSelect }: BylawListProps) {
  const groups = useMemo(() => buildGroups(bylaws), [bylaws])
  // Derive default open state from groups — re-computes when groups change
  const defaultOpen = useMemo(() => {
    return new Set(groups.filter(g => g.defaultOpen).map(g => g.id))
  }, [groups])
  const [manuallyToggled, setManuallyToggled] = useState<Set<string>>(new Set())
  const initialDefaultsRef = useRef<Set<string> | null>(null)

  // On first meaningful groups load, initialize manuallyToggled as empty
  // so openGroups = defaultOpen - manuallyToggled + manuallyToggled
  useEffect(() => {
    if (groups.length > 0 && initialDefaultsRef.current === null) {
      initialDefaultsRef.current = defaultOpen
    }
  }, [groups, defaultOpen])

  const openGroups = useMemo(() => {
    const result = new Set(defaultOpen)
    for (const id of groups.map(g => g.id)) {
      if (manuallyToggled.has(id)) {
        if (result.has(id)) result.delete(id)
        else result.add(id)
      }
    }
    return result
  }, [defaultOpen, manuallyToggled, groups])

  function toggleGroup(id: string) {
    setManuallyToggled(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  return (
    <nav className="bylaw-list" aria-label="Bylaw sections">
      {groups.map((group) => {
        const isOpen = openGroups.has(group.id)
        return (
          <div key={group.id} className="domain-group">
            <button
              className="domain-group-header"
              onClick={() => toggleGroup(group.id)}
              aria-expanded={isOpen}
            >
              <svg className={`chevron ${isOpen ? 'open' : ''}`} width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                <path d="m9 18 6-6-6-6" />
              </svg>
              <span>{group.label}</span>
              <span className="domain-badge">{group.sections.length}</span>
            </button>
            {isOpen && (
              <ul className="domain-group-items">
                {group.sections.map((bylaw) => (
                  <li key={bylaw.id}>
                    <button
                      className={`bylaw-list-item ${selectedId === bylaw.id ? 'selected' : ''}`}
                      onClick={() => onSelect(bylaw.id)}
                    >
                      {bylaw.article && (
                        <span className="bylaw-list-article">
                          {bylaw.domain === 'llpoa' ? `Art. ${bylaw.article}` : bylaw.article}
                        </span>
                      )}
                      <span className="bylaw-list-title-text">{cleanTitle(bylaw.title)}</span>
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>
        )
      })}
    </nav>
  )
}

function cleanTitle(title: string): string {
  return title
    .replace(/^(ARTICLE\s+\w+\s*[-–—]\s*)/i, '')
    .replace(/\.txt$/i, '')
    .trim()
}
