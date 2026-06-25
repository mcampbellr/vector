import { useState } from 'react'
import { ChevronRight, ChevronDown, Copy, Check } from 'lucide-react'
import type { Status } from '../../types/board'
import { nextCommandFor } from './nextCommandFor'
import styles from './NextCommand.module.css'

interface NextCommandProps {
  status: Status
  id: string
}

export function NextCommand({ status, id }: NextCommandProps) {
  const [expanded, setExpanded] = useState(false)
  const [copied, setCopied] = useState(false)

  const command = nextCommandFor(status, id)

  if (command === null) {
    return null
  }

  function handleCopy() {
    if (!navigator.clipboard) return
    navigator.clipboard.writeText(command!).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    })
  }

  return (
    <div className={styles.root}>
      <button
        type="button"
        className={styles.toggle}
        aria-expanded={expanded}
        onClick={() => setExpanded((prev) => !prev)}
      >
        {expanded ? (
          <ChevronDown size={12} strokeWidth={2.5} />
        ) : (
          <ChevronRight size={12} strokeWidth={2.5} />
        )}
        Next command
      </button>

      {expanded && (
        <div className={styles.body}>
          <code className={styles.command}>{command}</code>
          <button
            type="button"
            className={`${styles.copyBtn}${copied ? ` ${styles.copied}` : ''}`}
            aria-label="Copy command"
            onClick={handleCopy}
          >
            {copied ? (
              <Check size={12} strokeWidth={2.5} />
            ) : (
              <Copy size={12} strokeWidth={2.5} />
            )}
          </button>
        </div>
      )}
    </div>
  )
}
