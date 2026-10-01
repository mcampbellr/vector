import type { Status } from '../../types/board'
import { nextCommandFor } from '../SpecCard/nextCommandFor'

export interface NextStepRow {
  label: string
  command: string
  /** A shell line pasted in a terminal, not a slash command typed into Claude Code. */
  shell: boolean
}

/**
 * Explains the single-row case: a terminal spec has nothing left to tell Claude,
 * but `vector open` still opens its worktree (with a plain shell, no command).
 */
export const NO_CLAUDE_STEP_NOTE =
  'Nothing to tell Claude: the spec is closed. The terminal line opens a plain shell in its worktree.'

/** The shell line that opens (or focuses) the spec's tmux window. */
export function openCommandFor(id: string): string {
  return `vector open ${id}`
}

/**
 * Returns the ways to take the spec's next step, by destination — not as a
 * sequence. `vector open` already launches Claude with the slash command
 * (cli/cmd/vector/open_tmux.go), so the two rows are alternative doors to the
 * same step: running both would apply the spec twice. The terminal row comes
 * first and always applies (a closed spec still has a worktree to open); the
 * Claude row only exists while the spec has a next command.
 */
export function nextStepRows(status: Status, id: string): NextStepRow[] {
  const rows: NextStepRow[] = [{ label: 'From a terminal', command: openCommandFor(id), shell: true }]
  const nextCommand = nextCommandFor(status, id)
  if (nextCommand !== null) {
    rows.push({ label: 'Inside Claude Code', command: nextCommand, shell: false })
  }
  return rows
}
