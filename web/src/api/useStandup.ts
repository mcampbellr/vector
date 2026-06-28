import type { SpecActivity, StandupDigest } from '../types/standup'
import { useFetchJSON, type AsyncState } from './useFetchJSON'

// useStandup fetches the persisted standup digest. A never-run standup returns
// `{}`, which surfaces as a non-null digest with no `global` — the empty state.
export function useStandup(): AsyncState<StandupDigest> {
  return useFetchJSON<StandupDigest>('/api/standup')
}

// useSpecActivity fetches a spec's timeline. Pass null to skip the fetch (e.g.
// while a card's timeline is collapsed), keeping it lazy per card.
export function useSpecActivity(specId: string | null, since = '7d'): AsyncState<SpecActivity> {
  const url = specId ? `/api/activity?spec=${encodeURIComponent(specId)}&since=${since}` : null
  return useFetchJSON<SpecActivity>(url)
}
