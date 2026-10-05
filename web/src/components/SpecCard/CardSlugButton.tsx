import type { MouseEvent } from 'react'
import { Check, Copy } from 'lucide-react'
import { useCopyFeedback } from '../../lib/useCopyFeedback'
import styles from './SpecCard.module.css'

interface CardSlugButtonProps {
  slug: string
}

// CardSlugButton is the slug row of the Ledger card: the slug *is* the copy
// target, so the separate copy button is gone — it was a small target competing
// with the whole card. The Copy glyph appears on hover only, and on copy the
// datum is replaced in place by "copied" (no global toast).
// stopPropagation keeps the copy from also opening the details drawer.
export function CardSlugButton({ slug }: CardSlugButtonProps) {
  const { copied, copy } = useCopyFeedback()

  function handleCopy(event: MouseEvent<HTMLButtonElement>) {
    event.stopPropagation()
    copy(slug)
  }

  return (
    <button
      type="button"
      className={`${styles.slug}${copied ? ` ${styles.slugCopied}` : ''}`}
      title="Copy the slug"
      aria-label={`Copy spec id ${slug}`}
      onClick={handleCopy}
    >
      {copied ? (
        <>
          <Check size={11} strokeWidth={2.5} className={styles.slugGlyphFixed} />
          <span className={styles.slugText}>copied</span>
        </>
      ) : (
        <>
          <span className={styles.slugText}>{slug}</span>
          <Copy size={11} strokeWidth={2} className={styles.slugGlyph} />
        </>
      )}
    </button>
  )
}
