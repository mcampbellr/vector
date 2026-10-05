import type { Card } from '../../types/board'
import { CopyableCommand } from './CopyableCommand'
import { NO_CLAUDE_STEP_NOTE, nextStepRows } from './nextStepRows'
import styles from './SpecDetailsDrawer.module.css'

interface NextStepProps {
  card: Pick<Card, 'status' | 'id' | 'pr'>
}

// NextStep is the drawer's primary section: the spec's next step, one row per
// destination — the shell line for a terminal and the slash command for Claude
// Code. They are peers, not steps 1 and 2 (`vector open` starts Claude with that
// same command), so neither outranks the other. The section always renders: a
// closed spec keeps the terminal row and explains why the Claude row is gone.
// The Claude row follows nextCommandFor, so a review card with no recorded PR
// points at /vector:ship rather than /vector:close.
export function NextStep({ card }: NextStepProps) {
  const rows = nextStepRows(card)
  const hasClaudeRow = rows.some((row) => !row.shell)

  return (
    <section className={styles.section} aria-label="Next step">
      <h3 className={styles.sectionTitle}>Next step</h3>
      <div className={styles.cmdList}>
        {rows.map((row) => (
          <CopyableCommand key={row.command} label={row.label} command={row.command} shell={row.shell} />
        ))}
      </div>
      {!hasClaudeRow && <p className={styles.cmdEmpty}>{NO_CLAUDE_STEP_NOTE}</p>}
    </section>
  )
}
