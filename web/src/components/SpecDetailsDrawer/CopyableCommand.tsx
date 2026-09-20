import { Check, Copy } from 'lucide-react'
import { useCopyFeedback } from '../../lib/useCopyFeedback'
import styles from './SpecDetailsDrawer.module.css'

interface CopyableCommandProps {
  label: string
  command: string
}

// CopyableCommand is the flat, always-visible copy affordance used in the
// drawer's useful-commands list — the same copy-to-clipboard pattern as
// NextCommand, without the collapse (the drawer has room to show every command).
export function CopyableCommand({ label, command }: CopyableCommandProps) {
  const { copied, copy } = useCopyFeedback()

  function handleCopy() {
    copy(command)
  }

  return (
    <div className={styles.cmdRow}>
      <div className={styles.cmdText}>
        <span className={styles.cmdLabel}>{label}</span>
        <code className={styles.cmdCode}>{command}</code>
      </div>
      <button
        type="button"
        className={`${styles.cmdCopy}${copied ? ` ${styles.copied}` : ''}`}
        aria-label={`Copy command: ${command}`}
        onClick={handleCopy}
      >
        {copied ? <Check size={13} strokeWidth={2.5} /> : <Copy size={13} strokeWidth={2.5} />}
      </button>
    </div>
  )
}
