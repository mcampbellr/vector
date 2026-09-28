import { useId } from 'react'
import type { ChangeEvent } from 'react'
import { assignSpecEpic } from '../../api/boardWrites'
import { useWriteAction } from '../../api/useWriteAction'
import type { EpicSummary } from '../../types/board'
import styles from './SpecDetailsDrawer.module.css'

interface EpicAssignSelectProps {
  specId: string
  /** The spec's current epic id ('' / undefined = none). */
  epicId?: string
  epics: EpicSummary[]
}

const NO_EPIC = ''

// EpicAssignSelect moves a spec into an epic, or out of it, through
// POST /api/specs/{id}/epic. The select always shows the board's value (no
// optimistic copy); while the request is in flight it is disabled, and a
// failure is shown under it. A dangling epic id (epic deleted by hand) stays
// selectable so the current state is never misrepresented.
export function EpicAssignSelect({ specId, epicId, epics }: EpicAssignSelectProps) {
  const selectId = useId()
  const { run, pending, error } = useWriteAction(assignSpecEpic)
  const current = epicId ?? NO_EPIC
  const dangling = current !== NO_EPIC && !epics.some((epic) => epic.id === current)

  function handleChange(event: ChangeEvent<HTMLSelectElement>) {
    const next = event.target.value
    void run(specId, next === NO_EPIC ? null : next)
  }

  return (
    <div className={styles.epicAssign}>
      <select
        id={selectId}
        className={styles.epicSelect}
        aria-label="Epic"
        value={current}
        onChange={handleChange}
        disabled={pending}
        aria-busy={pending || undefined}
      >
        <option value={NO_EPIC}>no epic</option>
        {dangling && <option value={current}>{current} (missing)</option>}
        {epics.map((epic) => (
          <option key={epic.id} value={epic.id}>
            {epic.title}
          </option>
        ))}
      </select>
      {pending && <span className={styles.muted}>saving…</span>}
      {error && (
        <p className={styles.error} role="alert">
          could not change the epic: {error}
        </p>
      )}
      {epics.length === 0 && !dangling && (
        <p className={styles.muted}>No epics yet — create one in the epics tab.</p>
      )}
    </div>
  )
}
