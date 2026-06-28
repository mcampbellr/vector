import { Suspense, lazy, useEffect, useRef } from 'react'
import { X } from 'lucide-react'
import { useFileContent, type ArtifactKey } from '../../api/useFileContent'
import styles from './SpecDetailsDrawer.module.css'

// MarkdownView (and its react-markdown dependency) is code-split out of the
// initial board bundle and only loaded when a file is actually previewed.
const MarkdownView = lazy(() => import('./MarkdownView'))

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
export function FilePreviewModal({ specId, artifact, fileName, onClose }: FilePreviewModalProps) {
  const { data, loading, error, reload } = useFileContent(specId, artifact)
  const closeRef = useRef<HTMLButtonElement>(null)

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
    <div className={styles.modalOverlay} onClick={onClose}>
      <div
        className={styles.modalPanel}
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
      </div>
    </div>
  )
}
