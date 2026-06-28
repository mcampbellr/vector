import type { Card } from '../../types/board'

export interface UsefulCommand {
  label: string
  command: string
}

/**
 * Returns the context-aware slash commands worth surfacing for a card, beyond
 * its primary "next command" (which the drawer shows separately via NextCommand).
 * The set is gated by the spec's state-machine legality and its current
 * metadata: `/vector:link` appears only when the spec has no ticket; the status
 * moves shown are the legal ones for the current status. The board stays
 * read-only — these are copyable commands, never web mutations.
 */
export function usefulCommandsFor(card: Card): UsefulCommand[] {
  const { id, status } = card
  const commands: UsefulCommand[] = []

  // Assign a ticket from here — copyable, not a web mutation — only when unlinked.
  if (!card.ticket) {
    commands.push({ label: 'Link a ticket', command: `/vector:link ${id} <ticket-ref>` })
  }

  switch (status) {
    case 'in-progress':
      commands.push({ label: 'Send to review', command: `/vector:status ${id} review` })
      commands.push({ label: 'Flag blocked', command: `/vector:status ${id} needs-attention "<why>"` })
      break
    case 'needs-attention':
      commands.push({ label: 'Resume work', command: `/vector:status ${id} in-progress` })
      break
    case 'review':
      commands.push({ label: 'Reopen', command: `/vector:status ${id} in-progress` })
      break
    case 'closed':
      commands.push({ label: 'Archive', command: `/vector:archive ${id}` })
      break
  }

  return commands
}
