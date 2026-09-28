import { useCallback, useState } from 'react'
import { ALL_EPICS, EPIC_FILTER_PARAM, parseEpicFilter, serializeEpicFilter } from './epicFilter'
import type { EpicFilter } from './epicFilter'

function readInitialFilter(): EpicFilter {
  try {
    return parseEpicFilter(window.location.search)
  } catch {
    return ALL_EPICS
  }
}

// writeFilterToUrl mirrors the filter into the query string without adding a
// history entry. Wrapped in try/catch: a sandboxed or opaque-origin frame can
// throw on history access, and losing persistence must never break the board.
function writeFilterToUrl(filter: EpicFilter) {
  try {
    const url = new URL(window.location.href)
    const value = serializeEpicFilter(filter)
    if (value === null) url.searchParams.delete(EPIC_FILTER_PARAM)
    else url.searchParams.set(EPIC_FILTER_PARAM, value)
    window.history.replaceState(window.history.state, '', url)
  } catch {
    /* persistence is a convenience; keep the in-memory filter */
  }
}

// useEpicFilter owns the board's epic filter: initialised from `?epic=` and kept
// in sync with it on every change.
export function useEpicFilter(): [EpicFilter, (filter: EpicFilter) => void] {
  const [filter, setFilterState] = useState<EpicFilter>(readInitialFilter)

  const setFilter = useCallback((next: EpicFilter) => {
    setFilterState(next)
    writeFilterToUrl(next)
  }, [])

  return [filter, setFilter]
}
