/**
 * Shortens a ticket key to what the card shows: `mcampbellr/cdr-monorepo#174` →
 * `#174`, a bare number `174` → `#174`, and a provider key like `MH-1814` is
 * already short enough to keep whole. The full reference stays in the tooltip.
 *
 * The ref is rendered at a fixed intrinsic width, so it can never push the title
 * the way the old owner/repo#number badge did.
 */
export function shortTicketRef(key: string): string {
  const trailingNumber = key.match(/#(\d+)\s*$/)
  if (trailingNumber) return `#${trailingNumber[1]}`
  return /^\d+$/.test(key) ? `#${key}` : key
}
