import type { EpicColor } from '../types/board'

// Board writes — the only mutations the panel sends. Each is an intent the
// cli/ API persists through the same Store mutators the CLI uses
// (architecture/state-model.md); the panel never patches its own copy of the
// board. The fresh board arrives over the existing /api/events SSE stream, which
// the server pushes right after every successful write.

/** The epic fields the server echoes back from POST /api/epics and PATCH
 *  /api/epics/{id}; mirrors Go state.Epic. */
export interface EpicRecord {
  schemaVersion: number
  id: string
  title: string
  description?: string
  color?: EpicColor
  order?: number
  focus?: boolean
  createdAt: string
  updatedAt: string
}

export interface FocusResult {
  id: string
  focus: boolean
  changed: boolean
}

export interface EpicAssignResult {
  id: string
  /** '' when the spec no longer belongs to an epic. */
  epic: string
  changed: boolean
}

export interface NewEpicInput {
  title: string
  description?: string
  color?: EpicColor
  /** 1 = first; 0/absent = unordered. */
  order?: number
}

export interface EpicPatch {
  title?: string
  description?: string
  /** '' clears the color. */
  color?: EpicColor | ''
  /** 0 unsets the order. */
  order?: number
}

// sendJSON issues a same-origin JSON write and returns the parsed body, or throws
// with the server's {error} message (4xx/5xx) so callers can show it inline.
async function sendJSON<T>(method: 'POST' | 'PATCH', url: string, body: object): Promise<T> {
  const response = await fetch(url, {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!response.ok) {
    let message = `request failed (${response.status})`
    try {
      const payload = (await response.json()) as { error?: string }
      if (payload.error) message = payload.error
    } catch {
      /* non-JSON error body — keep the status message */
    }
    throw new Error(message)
  }
  return (await response.json()) as T
}

export function setSpecFocus(specId: string, focus: boolean): Promise<FocusResult> {
  return sendJSON<FocusResult>('POST', `/api/specs/${encodeURIComponent(specId)}/focus`, { focus })
}

/** Assigns the spec to epicId, or clears its epic when epicId is null. */
export function assignSpecEpic(specId: string, epicId: string | null): Promise<EpicAssignResult> {
  return sendJSON<EpicAssignResult>('POST', `/api/specs/${encodeURIComponent(specId)}/epic`, {
    epic: epicId,
  })
}

/** Toggles an epic's focus (POST /api/epics/{id}/focus); its non-closed specs
 *  inherit it on the next board push. */
export function setEpicFocus(epicId: string, focus: boolean): Promise<FocusResult> {
  return sendJSON<FocusResult>('POST', `/api/epics/${encodeURIComponent(epicId)}/focus`, { focus })
}

export function createEpic(input: NewEpicInput): Promise<EpicRecord> {
  return sendJSON<EpicRecord>('POST', '/api/epics', input)
}

export function updateEpic(epicId: string, patch: EpicPatch): Promise<EpicRecord> {
  return sendJSON<EpicRecord>('PATCH', `/api/epics/${encodeURIComponent(epicId)}`, patch)
}
