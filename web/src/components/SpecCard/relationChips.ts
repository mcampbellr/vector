import type { RelatedItem } from '../../types/board'

// RelationChip is the view-ready shape RelatedChips renders: a stable key, the
// short visible label (the cause ref) and a spelled-out accessible description.
// Keeping this derivation pure lets it be unit-tested without a DOM renderer
// (mirrors nextCommandFor / entries — behavior tests, not empty snapshots).
export interface RelationChip {
  key: string
  label: string
  ariaLabel: string
  title: string
}

// relationChips maps a spec's relations to the chips shown on its card. An empty
// or missing list yields no chips (the card omits the row entirely).
export function relationChips(related: RelatedItem[] | undefined): RelationChip[] {
  if (!related) return []
  return related.map((relation) => ({
    key: `${relation.kind}:${relation.ref}`,
    label: relation.ref,
    ariaLabel: `Caused by ${relation.kind} ${relation.ref} (${relation.source})`,
    title: `Caused by ${relation.kind} ${relation.ref} · ${relation.source}`,
  }))
}
