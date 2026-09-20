/**
 * Clips a card title to the two lines the Ledger card reserves for it, at a word
 * boundary, appending an ellipsis.
 *
 * The CSS `-webkit-line-clamp` on `.title` stays as a safety net, but it does not
 * paint its ellipsis in every engine, so the visible cut is decided here and the
 * untruncated title lives in the element's `title` attribute.
 *
 * `max` is the character budget for two lines of the card's text column: 54 with
 * a ticket ref sharing the first row, 62 without one.
 *
 * ---
 * DIVERGENCE FROM THE DESIGN SOURCE — deliberate, do not "restore" it.
 *
 * This is a port of the `clip()` helper in the Claude Design file "Vector Board -
 * Card rules" (project 82ab48de-3a1e-428c-b0f9-b80597e65a72). That original runs
 * its long-word rule BEFORE the length check, which truncates titles that fit the
 * budget with room to spare:
 *
 *   "Add google-services.json to the Android build"  (45 chars, budget 62)
 *     → "Add google-services.json…"
 *
 * In a repo whose titles are full of package names, file names and identifiers,
 * that silently eats the verb and object of a large share of the board. The order
 * is inverted here: the length check comes first, and the long-word rule applies
 * only once the text actually overflows. That serves the design's own stated
 * rationale — the rule exists because "anything after it would spill to a third
 * line", which is only true when the text overflows in the first place.
 *
 * The design file still carries the original order; porting it again verbatim
 * would reintroduce the bug.
 */
export function clipTitle(text: string, max: number): string {
  if (text.length <= max) return text

  // A word longer than ~18 characters (PersistQueryClientProvider,
  // google-services.json) eats a whole line on its own, so anything after it
  // would spill onto a third line: cut right there.
  const words = text.split(' ')
  const long = words.findIndex((word) => word.length > 18)
  if (long >= 0 && words.length > long + 1) return withEllipsis(words.slice(0, long + 1).join(' '))

  const cut = text.slice(0, max)
  const lastSpace = cut.lastIndexOf(' ')
  // Only honour the word boundary when it is not so early that the cut would
  // throw away most of the budget; otherwise clip mid-word.
  return withEllipsis(lastSpace > max * 0.6 ? cut.slice(0, lastSpace) : cut)
}

function withEllipsis(text: string): string {
  return `${text.replace(/[\s,.;:—+-]+$/, '')}…`
}
