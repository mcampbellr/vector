import { Check, Copy } from 'lucide-react'
import { useCopyFeedback } from '../../lib/useCopyFeedback'
import styles from './SpecDetailsDrawer.module.css'

interface CopyableCommandProps {
  label: string
  command: string
  /**
   * A shell line rather than a slash command: it gets a `$` prompt marker and a
   * clipboard-specific aria-label. Slash commands stay unmarked — they are the
   * board's default and carry no new badge.
   */
  shell?: boolean
}

// CopyableCommand is the flat, always-visible copy affordance used in the
// drawer's command sections — the same copy-to-clipboard pattern as
// NextCommand, without the collapse (the drawer has room to show every command).
// The `$` marker is decorative: it is aria-hidden, unselectable, and never part
// of what the click copies (the clipboard always gets `command` verbatim).
export function CopyableCommand({ label, command, shell = false }: CopyableCommandProps) {
  const { copied, copy } = useCopyFeedback()

  function handleCopy() {
    copy(command)
  }

  return (
    <div className={styles.cmdRow}>
      <div className={styles.cmdText}>
        <span className={styles.cmdLabel}>{label}</span>
        {/* The chip ellipsises a long line; title carries it whole. */}
        <code className={styles.cmdCode} title={command}>
          {shell && (
            <span className={styles.cmdShellPrefix} aria-hidden="true">
              $
            </span>
          )}
          {command}
        </code>
      </div>
      <button
        type="button"
        className={`${styles.cmdCopy}${copied ? ` ${styles.copied}` : ''}`}
        aria-label={`${shell ? 'Copy shell command' : 'Copy command'}: ${command}`}
        onClick={handleCopy}
      >
        {copied ? <Check size={13} strokeWidth={2.5} /> : <Copy size={13} strokeWidth={2.5} />}
      </button>
    </div>
  )
}
