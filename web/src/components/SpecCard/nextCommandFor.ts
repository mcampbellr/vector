import type { Status } from '../../types/board'

/**
 * Returns the slash command the user should run next for a given card status,
 * or null when no command applies (closed = terminal state).
 */
export function nextCommandFor(status: Status, id: string): string | null {
  switch (status) {
    case 'draft':
      return `/vector:propose ${id}`
    case 'open':
    case 'in-progress':
    case 'needs-attention':
      return `/vector:apply ${id}`
    case 'review':
      return `/vector:close ${id}`
    case 'closed':
      return null
  }
}
