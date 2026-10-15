import { Hourglass } from 'lucide-react'
import type { CardAge } from './cardAge'
import styles from './SpecCard.module.css'

interface CardAgeStampProps {
  age: CardAge
}

// CardAgeStamp prints how long ago a spec was touched, in the identity row and
// right-aligned against the artifact meter — so every card in a column puts the
// datum at the same x and a column can be read down that one line.
//
// The encoding is luminance and weight, not hue: a bright, bold token among dim
// mono pops before it is read, and a column of 8 cards stays calm. Hue is spent
// only on the last step, so a healthy column carries zero or one red mark.
//
// It is a plain span: not focusable, not clickable. The absolute timestamp rides
// in the native `title` rather than replacing the text on hover, because a full
// date is about four times wider and would re-truncate the slug under the
// pointer. Keyboard and screen-reader users get the age from the card's own
// aria-label, which SpecCard composes.
export function CardAgeStamp({ age }: CardAgeStampProps) {
  const toneClass =
    age.tone === 'very-stale'
      ? styles.ageVeryStale
      : age.tone === 'stale'
        ? styles.ageStale
        : ''

  return (
    <span className={`${styles.age}${toneClass ? ` ${toneClass}` : ''}`} title={age.tooltip}>
      {age.tone === 'very-stale' && (
        <Hourglass className={styles.ageGlyph} size={11} strokeWidth={2} aria-hidden="true" />
      )}
      {age.text}
    </span>
  )
}
