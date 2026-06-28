import type { Card } from '../../types/board'
import { CopyableCommand } from './CopyableCommand'
import { usefulCommandsFor } from './usefulCommandsFor'
import styles from './SpecDetailsDrawer.module.css'

interface UsefulCommandsProps {
  card: Card
}

// UsefulCommands renders the drawer's context-aware list of copyable slash
// commands for a spec (link/status/close/archive, gated by legality). It renders
// nothing when no extra command applies beyond the primary next command.
export function UsefulCommands({ card }: UsefulCommandsProps) {
  const commands = usefulCommandsFor(card)
  if (commands.length === 0) return null

  return (
    <section className={styles.section}>
      <h3 className={styles.sectionTitle}>Useful commands</h3>
      <div className={styles.cmdList}>
        {commands.map((cmd) => (
          <CopyableCommand key={cmd.command} label={cmd.label} command={cmd.command} />
        ))}
      </div>
    </section>
  )
}
