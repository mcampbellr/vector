import type { MouseEvent } from 'react'
import { Check } from 'lucide-react'
import type { Card } from '../../types/board'
import { useCopyFeedback } from '../../lib/useCopyFeedback'
import { nextCommandFor, nextVerbFor } from './nextCommandFor'
import styles from './SpecCard.module.css'

interface CardVerbButtonProps {
  card: Pick<Card, 'status' | 'id' | 'pr'>
}

// CardVerbButton collapses the next slash command to its verb: `/vector:ship
// <slug>` renders as `ship`, the tooltip carries the whole line and the click
// copies it. The verb depends on status and, in review, on whether a PR is
// recorded (ship without one, close with one). Renders nothing for a closed
// spec — a terminal card has no next move, so it has no verb.
export function CardVerbButton({ card }: CardVerbButtonProps) {
  const { copied, copy } = useCopyFeedback()

  const command = nextCommandFor(card)
  if (command === null) return null

  const verb = nextVerbFor(card)

  // An arrow, not a function declaration: TypeScript keeps the `command !== null`
  // narrowing inside a closure created after the check, but a hoisted declaration
  // falls back to the declared `string | null`.
  const handleCopy = (event: MouseEvent<HTMLButtonElement>) => {
    event.stopPropagation()
    // The click copies the whole line, not the verb the button shows.
    copy(command)
  }

  return (
    <button
      type="button"
      className={`${styles.verb}${copied ? ` ${styles.verbCopied}` : ''}`}
      title={command}
      aria-label={`Copy next command: ${command}`}
      onClick={handleCopy}
    >
      {copied && <Check size={11} strokeWidth={2.5} />}
      {copied ? 'copied' : verb}
    </button>
  )
}
