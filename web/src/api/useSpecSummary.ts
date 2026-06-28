import type { SpecSummary } from '../types/board'
import { useFetchJSON, type AsyncState } from './useFetchJSON'

// useSpecSummary fetches a spec's persisted post-action summary. Pass null to
// skip the fetch (e.g. while the details drawer is closed), keeping it lazy. A
// spec with no summary yet returns `{}`, surfacing as a non-null object with no
// `summary` — the empty state.
export function useSpecSummary(specId: string | null): AsyncState<SpecSummary> {
  const url = specId ? `/api/summary?spec=${encodeURIComponent(specId)}` : null
  return useFetchJSON<SpecSummary>(url)
}
