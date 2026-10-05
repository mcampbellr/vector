import type { FileErrorKind } from '../../api/useFileContent'
import styles from './SpecDetailsDrawer.module.css'

interface FilePreviewErrorProps {
  kind: FileErrorKind
  message: string
  onRetry: () => void
}

// FilePreviewError renders the modal body when a file fails to load. A 404 is
// terminal — the server found no surviving copy (its worktree was removed and
// nothing was archived or snapshotted) — so it offers no Retry. An unreachable
// server (fetch rejected) names the likely cause; any other failure keeps the
// generic message. Both retryable states keep the Retry action.
export function FilePreviewError({ kind, message, onRetry }: FilePreviewErrorProps) {
  if (kind === 'not-found') {
    return (
      <div className={styles.modalError}>
        <p className={styles.muted}>file not available — its source was removed and no copy was found.</p>
      </div>
    )
  }
  return (
    <div className={styles.modalError}>
      <p className={styles.error}>
        {kind === 'unreachable'
          ? 'could not reach the vector server — is `vector serve` still running?'
          : `could not load file: ${message}`}
      </p>
      <button type="button" className={styles.retry} onClick={onRetry}>
        Retry
      </button>
    </div>
  )
}
