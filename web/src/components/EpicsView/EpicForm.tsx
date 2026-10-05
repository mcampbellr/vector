import { useId, useState } from 'react'
import type { FormEvent } from 'react'
import { createEpic, updateEpic } from '../../api/boardWrites'
import type { EpicRecord, NewEpicInput } from '../../api/boardWrites'
import { useWriteAction } from '../../api/useWriteAction'
import type { EpicColor, EpicSummary } from '../../types/board'
import { EpicColorSwatches } from './EpicColorSwatches'
import { parseOrderInput } from './helpers'
import styles from './EpicsView.module.css'

interface EpicFormProps {
  /** Absent: create a new epic. Present: edit this one (PATCH). */
  epic?: EpicSummary
  onDone: () => void
  onCancel: () => void
}

// saveEpic is the form's single write: create, or patch the edited epic with
// every field (so clearing the description/colour sticks).
function saveEpic(existingId: string | null, input: NewEpicInput): Promise<EpicRecord> {
  if (existingId === null) return createEpic(input)
  return updateEpic(existingId, {
    title: input.title,
    description: input.description ?? '',
    color: input.color ?? '',
    order: input.order ?? 0,
  })
}

// EpicForm creates an epic (POST /api/epics) or edits one (PATCH
// /api/epics/{id}): title (required), optional description, palette colour and
// order (1 = first; empty = unordered, sorted after the ordered epics by title).
// Inputs are disabled while the request is pending; the server's validation
// message is shown inline. The new epic appears through the SSE board push.
export function EpicForm({ epic, onDone, onCancel }: EpicFormProps) {
  const formId = useId()
  const [title, setTitle] = useState(epic?.title ?? '')
  const [description, setDescription] = useState(epic?.description ?? '')
  const [color, setColor] = useState<EpicColor | ''>(epic?.color ?? '')
  const [orderText, setOrderText] = useState(epic?.order ? String(epic.order) : '')
  const { run, pending, error } = useWriteAction(saveEpic)
  const editing = epic !== undefined
  const order = parseOrderInput(orderText)

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (order === null) return
    const saved = await run(epic?.id ?? null, {
      title: title.trim(),
      description: description.trim() || undefined,
      color: color || undefined,
      order: order > 0 ? order : undefined,
    })
    if (saved) onDone()
  }

  return (
    <form
      className={styles.form}
      onSubmit={handleSubmit}
      aria-label={epic ? `Edit epic ${epic.title}` : 'New epic'}
    >
      <label className={styles.field} htmlFor={`${formId}-title`}>
        <span className={styles.fieldLabel}>title</span>
        <input
          id={`${formId}-title`}
          className={styles.input}
          value={title}
          onChange={(event) => setTitle(event.target.value)}
          placeholder="App Mobile"
          required
          disabled={pending}
          autoFocus
        />
      </label>
      <label className={styles.field} htmlFor={`${formId}-description`}>
        <span className={styles.fieldLabel}>description</span>
        <textarea
          id={`${formId}-description`}
          className={styles.textarea}
          value={description}
          onChange={(event) => setDescription(event.target.value)}
          placeholder="Optional — what the epic groups"
          rows={2}
          disabled={pending}
        />
      </label>
      <div className={styles.field}>
        <label className={styles.fieldLabel} htmlFor={`${formId}-order`}>
          order
        </label>
        <input
          id={`${formId}-order`}
          className={`${styles.input} ${styles.orderInput}`}
          value={orderText}
          onChange={(event) => setOrderText(event.target.value)}
          inputMode="numeric"
          placeholder="—"
          aria-describedby={`${formId}-order-hint`}
          aria-invalid={order === null || undefined}
          disabled={pending}
        />
        <span id={`${formId}-order-hint`} className={styles.fieldHint}>
          {order === null
            ? 'Use a whole number (1 = first), or leave empty.'
            : '1 = first. Empty = unordered (after the ordered epics).'}
        </span>
      </div>
      <div className={styles.field}>
        <span className={styles.fieldLabel}>color</span>
        <EpicColorSwatches name={`${formId}-color`} value={color} onChange={setColor} disabled={pending} />
      </div>
      {error && (
        <p className={styles.formError} role="alert">
          {error}
        </p>
      )}
      <div className={styles.formActions}>
        <button
          type="submit"
          className={styles.primaryButton}
          disabled={pending || title.trim() === '' || order === null}
        >
          {pending ? 'saving…' : editing ? 'save' : 'create epic'}
        </button>
        <button type="button" className={styles.secondaryButton} onClick={onCancel} disabled={pending}>
          cancel
        </button>
      </div>
    </form>
  )
}
