import { useState } from 'react'
import type { MouseEvent } from 'react'
import { Check, Copy } from 'lucide-react'
import type { Status } from '../../types/board'
import { nextCommandFor } from './nextCommandFor'
import styles from './CardNextCommand.module.css'

interface CardNextCommandProps {
  status: Status
  id: string
}

// CardNextCommand is the always-visible quick-copy row on the card face: the
// next slash command (derived from status via nextCommandFor) plus a
// copy-to-clipboard button. It mirrors the drawer's CopyableCommand pattern
// without the collapse. Returns null when no command applies (closed = terminal).
export function CardNextCommand({ status, id }: CardNextCommandProps) {
  const [copied, setCopied] = useState(false)

  const command = nextCommandFor(status, id)

  if (command === null) {
    return null
  }

  function handleCopy(event: MouseEvent<HTMLButtonElement>) {
    event.stopPropagation()
    if (!navigator.clipboard) return
    navigator.clipboard.writeText(command!).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    })
  }

  return (
    <div className={styles.body}>
      <code className={styles.command}>{command}</code>
      <button
        type="button"
        className={`${styles.copyBtn}${copied ? ` ${styles.copied}` : ''}`}
        aria-label="Copy next command"
        onClick={handleCopy}
      >
        {copied ? <Check size={12} strokeWidth={2.5} /> : <Copy size={12} strokeWidth={2.5} />}
      </button>
    </div>
  )
}
