import { useEffect, useState } from 'react'
import type { Board } from '../types/board'

export type ConnectionState = 'loading' | 'live' | 'reconnecting' | 'error'

interface BoardHook {
  board: Board | null
  connection: ConnectionState
  error: string | null
}

// useBoard subscribes to the live board over Server-Sent Events. The SSE stream
// emits the current board immediately on connect and again on every state
// change, so a separate initial fetch is unnecessary. The browser's EventSource
// reconnects automatically on drop.
export function useBoard(): BoardHook {
  const [board, setBoard] = useState<Board | null>(null)
  const [connection, setConnection] = useState<ConnectionState>('loading')
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const source = new EventSource('/api/events')

    source.addEventListener('board', (event) => {
      try {
        setBoard(JSON.parse((event as MessageEvent).data) as Board)
        setConnection('live')
        setError(null)
      } catch (parseError) {
        setError(parseError instanceof Error ? parseError.message : 'parse error')
      }
    })

    source.onopen = () => setConnection('live')

    source.onerror = () => {
      // EventSource retries on its own; reflect the gap without tearing down.
      setConnection((prev) => (prev === 'live' ? 'reconnecting' : 'error'))
    }

    return () => source.close()
  }, [])

  return { board, connection, error }
}
