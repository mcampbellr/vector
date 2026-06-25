import { useEffect, useState } from 'react'

// useNow re-renders every intervalMs so relative timestamps ("20 sec ago") stay
// current without a board push.
export function useNow(intervalMs = 1000): number {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), intervalMs)
    return () => clearInterval(id)
  }, [intervalMs])
  return now
}
