import { useCallback, useEffect, useState } from 'react'
import type { SpecActivity, StandupDigest } from '../types/standup'

interface AsyncState<T> {
  data: T | null
  loading: boolean
  error: string | null
  reload: () => void
}

// parseError pulls the {error} field the standup handlers return on 4xx/5xx,
// falling back to the status text.
async function parseError(res: Response): Promise<string> {
  try {
    const body = (await res.json()) as { error?: string }
    if (body.error) return body.error
  } catch {
    /* non-JSON body — fall through */
  }
  return `request failed (${res.status})`
}

function useFetchJSON<T>(url: string | null): AsyncState<T> {
  const [data, setData] = useState<T | null>(null)
  const [loading, setLoading] = useState<boolean>(url !== null)
  const [error, setError] = useState<string | null>(null)
  const [nonce, setNonce] = useState(0)

  const reload = useCallback(() => setNonce((n) => n + 1), [])

  useEffect(() => {
    if (url === null) {
      setData(null)
      setLoading(false)
      setError(null)
      return
    }
    let cancelled = false
    setLoading(true)
    setError(null)
    fetch(url)
      .then(async (res) => {
        if (!res.ok) throw new Error(await parseError(res))
        return (await res.json()) as T
      })
      .then((json) => {
        if (!cancelled) {
          setData(json)
          setLoading(false)
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'request failed')
          setLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [url, nonce])

  return { data, loading, error, reload }
}

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
