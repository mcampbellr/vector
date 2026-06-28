import { useState } from 'react'
import type { MouseEvent } from 'react'
import { Check, Copy } from 'lucide-react'
import styles from './CopyableSlug.module.css'

interface CopyableSlugProps {
  slug: string
}

// CopyableSlug is the always-visible, copyable spec slug shown under the title on
// the card face and in the details drawer header: the bare slug (card.id) as a
// compact monospace chip plus a copy-to-clipboard button. It mirrors the copy
// pattern of CardNextCommand (stopPropagation + Copy → Check feedback ~1.5s) so
// copying on the card does not also open the drawer. Presentational only — no
// fetch, no board mutation.
export function CopyableSlug({ slug }: CopyableSlugProps) {
  const [copied, setCopied] = useState(false)

  function handleCopy(event: MouseEvent<HTMLButtonElement>) {
    event.stopPropagation()
    if (!navigator.clipboard) return
    navigator.clipboard.writeText(slug).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    })
  }

  return (
    <div className={styles.row}>
      <code className={styles.slug}>{slug}</code>
      <button
        type="button"
        className={`${styles.copyBtn}${copied ? ` ${styles.copied}` : ''}`}
        aria-label="Copy spec id"
        onClick={handleCopy}
      >
        {copied ? <Check size={12} strokeWidth={2.5} /> : <Copy size={12} strokeWidth={2.5} />}
      </button>
    </div>
  )
}
