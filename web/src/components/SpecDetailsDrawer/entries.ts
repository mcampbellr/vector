import type { Card } from '../../types/board'
import type { ArtifactKey } from '../../api/useFileContent'

export interface ArtifactEntry {
  key: ArtifactKey
  label: string
}

// basename returns the trailing path segment for the spec-doc label, falling
// back to a generic name when the pointer is empty.
export function basename(path: string): string {
  const segments = path.split('/')
  return segments[segments.length - 1] || 'spec.md'
}

// entriesFor derives the available source documents purely from the already
// loaded Card — card.specDoc for the authored spec and the artifacts flags for
// the OpenSpec change. No fetch, no filesystem scan; content is fetched lazily
// when an entry is selected.
//
// specDoc is the authored spec (the 20-section doc from /vector:raw); the OpenSpec
// artifacts live under openspec/changes/<change>/ and are distinct files. propose
// never rewrites specDoc to proposal.md, so the spec doc is listed regardless of
// whether OpenSpec artifacts exist — otherwise non-draft cards lose access to it.
export function entriesFor(card: Card): ArtifactEntry[] {
  const entries: ArtifactEntry[] = []
  if (card.specDoc) {
    entries.push({ key: 'spec', label: basename(card.specDoc) })
  }
  if (card.artifacts?.proposal) entries.push({ key: 'proposal', label: 'proposal.md' })
  if (card.artifacts?.design) entries.push({ key: 'design', label: 'design.md' })
  if (card.artifacts?.tasks) entries.push({ key: 'tasks', label: 'tasks.md' })
  return entries
}
