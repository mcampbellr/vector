import { Suspense, lazy, useCallback, useEffect, useRef, useState } from 'react'
import { X } from 'lucide-react'
import { useFileContent, type ArtifactKey } from '../../api/useFileContent'
import styles from './SpecDetailsDrawer.module.css'

// MarkdownView (and its react-markdown dependency) is code-split out of the
// initial board bundle and only loaded when a file is actually previewed.
const MarkdownView = lazy(() => import('./MarkdownView'))

export const SIZE_STORAGE_KEY = 'vector:file-preview-modal:size'

const DEFAULT_WIDTH = 720
const DEFAULT_HEIGHT_RATIO = 0.86
const MAX_WIDTH_RATIO = 0.95
const MAX_HEIGHT_RATIO = 0.95

export interface ModalSize {
  width: number
  height: number
}

type ResizeAxis = 'right' | 'bottom' | 'corner'

// defaultSize is the floor: the modal can grow from here but never shrink below
// it. Height is a ratio of the viewport at the moment the modal mounts.
export function defaultSize(): ModalSize {
  return { width: DEFAULT_WIDTH, height: window.innerHeight * DEFAULT_HEIGHT_RATIO }
}

// clampSize keeps a size within [floor, ceiling]. The ceiling (~95vw/95vh) is
// recomputed from the live viewport on every call so the panel never escapes the
// screen, even if the browser window is resized mid-drag. The floor is clamped to
// the ceiling so a narrow viewport (< DEFAULT_WIDTH) still yields a valid size.
export function clampSize(size: ModalSize, floor: ModalSize): ModalSize {
  const maxWidth = window.innerWidth * MAX_WIDTH_RATIO
  const maxHeight = window.innerHeight * MAX_HEIGHT_RATIO
  const minWidth = Math.min(floor.width, maxWidth)
  const minHeight = Math.min(floor.height, maxHeight)
  return {
    width: Math.min(Math.max(size.width, minWidth), maxWidth),
    height: Math.min(Math.max(size.height, minHeight), maxHeight),
  }
}

// loadPersistedSize reads the global modal size from localStorage, validates the
// shape, and re-clamps it to the current viewport. Returns null on any failure
// (absent, malformed JSON, wrong types, storage unavailable) so the caller falls
// back to the default.
export function loadPersistedSize(floor: ModalSize): ModalSize | null {
  try {
    const raw = localStorage.getItem(SIZE_STORAGE_KEY)
    if (!raw) return null
    const parsed: unknown = JSON.parse(raw)
    if (typeof parsed !== 'object' || parsed === null) return null
    const { width, height } = parsed as Record<string, unknown>
    if (
      typeof width !== 'number' ||
      typeof height !== 'number' ||
      !Number.isFinite(width) ||
      !Number.isFinite(height)
    ) {
      return null
    }
    return clampSize({ width, height }, floor)
  } catch {
    return null
  }
}

// savePersistedSize persists the chosen size. Silent on failure (private mode,
// storage disabled): the modal keeps working in memory for the session.
export function savePersistedSize(size: ModalSize): void {
  try {
    localStorage.setItem(SIZE_STORAGE_KEY, JSON.stringify(size))
  } catch {
    /* storage unavailable — in-memory sizing only */
  }
}

function cursorFor(axis: ResizeAxis): string {
  if (axis === 'right') return 'col-resize'
  if (axis === 'bottom') return 'row-resize'
  return 'se-resize'
}

interface DragState {
  axis: ResizeAxis
  startX: number
  startY: number
  startWidth: number
  startHeight: number
}

interface FilePreviewModalProps {
  specId: string
  artifact: ArtifactKey
  fileName: string
  onClose: () => void
}

// FilePreviewModal stacks above the details drawer and renders one spec artifact
// as Markdown. It closes independently of the drawer (button / Escape / overlay
// click). Escape is handled here first and its propagation stopped so the
// drawer's own Escape-to-close does not also fire; with no modal open, Escape
// closes the drawer (existing behavior). Focus moves to the close button on open
// and is restored to the previously focused element on close (best-effort).
//
// The panel is resizable from its right edge (width), bottom edge (height), and
// bottom-right corner (both) via native DOM drag — no library, no CSS resize. The
// size is clamped to the default floor and a ~95vw/95vh ceiling, and persisted
// globally to localStorage so it survives reopens.
export function FilePreviewModal({ specId, artifact, fileName, onClose }: FilePreviewModalProps) {
  const { data, loading, error, reload } = useFileContent(specId, artifact)
  const closeRef = useRef<HTMLButtonElement>(null)
  // True only when a press began on the overlay itself. A resize drag presses a
  // handle (inside the panel) and releases over the overlay, which would fire a
  // synthetic click on the overlay and close the modal — this gate prevents that.
  const overlayPressRef = useRef(false)

  // Floor is captured once on mount and stays constant for this open session.
  const floorRef = useRef<ModalSize | null>(null)
  if (floorRef.current === null) floorRef.current = defaultSize()
  const floor = floorRef.current

  const [size, setSize] = useState<ModalSize>(() => loadPersistedSize(floor) ?? floor)

  // Latest size, drag origin, latest pointer, and pending rAF id — kept in refs so
  // the document-level listeners read fresh values without re-binding each render.
  const sizeRef = useRef<ModalSize>(size)
  const dragRef = useRef<DragState | null>(null)
  const pointRef = useRef<{ x: number; y: number } | null>(null)
  const frameRef = useRef<number | null>(null)

  useEffect(() => {
    sizeRef.current = size
  }, [size])

  const onMouseMove = useCallback(
    (event: MouseEvent) => {
      if (!dragRef.current) return
      pointRef.current = { x: event.clientX, y: event.clientY }
      if (frameRef.current !== null) return // a frame is already scheduled
      frameRef.current = requestAnimationFrame(() => {
        frameRef.current = null
        const drag = dragRef.current
        const point = pointRef.current
        if (!drag || !point) return
        const next: ModalSize = { width: drag.startWidth, height: drag.startHeight }
        if (drag.axis === 'right' || drag.axis === 'corner') {
          next.width = drag.startWidth + (point.x - drag.startX)
        }
        if (drag.axis === 'bottom' || drag.axis === 'corner') {
          next.height = drag.startHeight + (point.y - drag.startY)
        }
        setSize(clampSize(next, floor))
      })
    },
    [floor],
  )

  const onMouseUp = useCallback(() => {
    document.removeEventListener('mousemove', onMouseMove)
    document.removeEventListener('mouseup', onMouseUp)
    document.body.style.cursor = ''
    document.body.style.userSelect = ''
    if (frameRef.current !== null) {
      cancelAnimationFrame(frameRef.current)
      frameRef.current = null
    }
    dragRef.current = null
    savePersistedSize(sizeRef.current)
  }, [onMouseMove])

  const startResize = useCallback(
    (axis: ResizeAxis) => (event: React.MouseEvent<HTMLDivElement>) => {
      event.preventDefault() // block text selection during the drag
      dragRef.current = {
        axis,
        startX: event.clientX,
        startY: event.clientY,
        startWidth: sizeRef.current.width,
        startHeight: sizeRef.current.height,
      }
      pointRef.current = { x: event.clientX, y: event.clientY }
      document.body.style.cursor = cursorFor(axis)
      document.body.style.userSelect = 'none'
      document.addEventListener('mousemove', onMouseMove)
      document.addEventListener('mouseup', onMouseUp)
    },
    [onMouseMove, onMouseUp],
  )

  // Remove any orphaned listeners and restore the body styles if the modal
  // unmounts mid-drag (e.g. closed with Escape while dragging).
  useEffect(() => {
    return () => {
      document.removeEventListener('mousemove', onMouseMove)
      document.removeEventListener('mouseup', onMouseUp)
      if (frameRef.current !== null) cancelAnimationFrame(frameRef.current)
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
    }
  }, [onMouseMove, onMouseUp])

  useEffect(() => {
    const previouslyFocused = document.activeElement as HTMLElement | null
    closeRef.current?.focus()

    function onKey(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        event.stopPropagation()
        onClose()
      }
    }
    window.addEventListener('keydown', onKey, true)
    return () => {
      window.removeEventListener('keydown', onKey, true)
      previouslyFocused?.focus?.()
    }
  }, [onClose])

  return (
    <div
      className={styles.modalOverlay}
      onMouseDown={(event) => {
        overlayPressRef.current = event.target === event.currentTarget
      }}
      onClick={(event) => {
        if (event.target === event.currentTarget && overlayPressRef.current) onClose()
      }}
    >
      <div
        className={styles.modalPanel}
        style={{ width: `${size.width}px`, height: `${size.height}px` }}
        role="dialog"
        aria-modal="true"
        aria-label={fileName}
        onClick={(event) => event.stopPropagation()}
      >
        <header className={styles.modalHeader}>
          <code className={styles.modalFile}>{fileName}</code>
          <button
            ref={closeRef}
            type="button"
            className={styles.close}
            aria-label="Close file preview"
            onClick={onClose}
          >
            <X size={16} strokeWidth={2.5} />
          </button>
        </header>

        <div className={styles.modalBody}>
          {loading && <p className={styles.muted}>loading file…</p>}
          {error && (
            <div className={styles.modalError}>
              <p className={styles.error}>could not load file: {error}</p>
              <button type="button" className={styles.retry} onClick={reload}>
                Retry
              </button>
            </div>
          )}
          {!loading && !error && data !== null && (
            <div className={styles.markdown}>
              <Suspense fallback={<p className={styles.muted}>loading file…</p>}>
                <MarkdownView source={data} />
              </Suspense>
            </div>
          )}
        </div>

        <div
          className={styles.resizeHandleRight}
          role="separator"
          aria-orientation="vertical"
          aria-label="Redimensionar ancho"
          tabIndex={0}
          onMouseDown={startResize('right')}
        />
        <div
          className={styles.resizeHandleBottom}
          role="separator"
          aria-orientation="horizontal"
          aria-label="Redimensionar alto"
          tabIndex={0}
          onMouseDown={startResize('bottom')}
        />
        <div
          className={styles.resizeHandleCorner}
          role="separator"
          aria-label="Redimensionar modal"
          tabIndex={0}
          onMouseDown={startResize('corner')}
        />
      </div>
    </div>
  )
}
