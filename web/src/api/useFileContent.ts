import { useCallback, useEffect, useState } from 'react'

/** The artifact keys the /api/file endpoint accepts (mirrors the Go enum). */
export type ArtifactKey = 'spec' | 'proposal' | 'design' | 'tasks'

export interface FileContentState {
  data: string | null
  loading: boolean
  error: string | null
  reload: () => void
}

// parseError pulls the {error} field the local API returns on 4xx/5xx, falling
// back to the status text. The body is JSON only on errors; success is raw
// Markdown, so this never runs on the happy path.
async function parseError(res: Response): Promise<string> {
  try {
    const body = (await res.json()) as { error?: string }
    if (body.error) return body.error
  } catch {
    /* non-JSON body — fall through */
  }
  return `request failed (${res.status})`
}

// useFileContent fetches a spec artifact as raw Markdown text from
// GET /api/file?spec=&artifact=. It mirrors useFetchJSON's {data, loading,
// error, reload} contract but resolves res.text() instead of JSON, because the
// endpoint serves a Markdown body, not an envelope. Pass a null artifact to keep
// it idle (lazy) — the modal only fetches the file the user selected.
export function useFileContent(spec: string | null, artifact: ArtifactKey | null): FileContentState {
  const active = spec !== null && artifact !== null
  const [data, setData] = useState<string | null>(null)
  const [loading, setLoading] = useState<boolean>(active)
  const [error, setError] = useState<string | null>(null)
  const [nonce, setNonce] = useState(0)

  const reload = useCallback(() => setNonce((n) => n + 1), [])

  useEffect(() => {
    if (!active) {
      setData(null)
      setLoading(false)
      setError(null)
      return
    }
    let cancelled = false
    setLoading(true)
    setError(null)
    const url = `/api/file?spec=${encodeURIComponent(spec)}&artifact=${encodeURIComponent(artifact)}`
    fetch(url)
      .then(async (res) => {
        if (!res.ok) throw new Error(await parseError(res))
        return res.text()
      })
      .then((text) => {
        if (!cancelled) {
          setData(text)
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
  }, [active, spec, artifact, nonce])

  return { data, loading, error, reload }
}
