import { useState } from 'react'
import { Check, Copy } from 'lucide-react'
import styles from './SpecDetailsDrawer.module.css'

interface CopyableCommandProps {
  label: string
  command: string
}

// CopyableCommand is the flat, always-visible copy affordance used in the
// drawer's useful-commands list — the same copy-to-clipboard pattern as
// NextCommand, without the collapse (the drawer has room to show every command).
export function CopyableCommand({ label, command }: CopyableCommandProps) {
  const [copied, setCopied] = useState(false)

  function handleCopy() {
    if (!navigator.clipboard) return
    navigator.clipboard.writeText(command).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    })
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
