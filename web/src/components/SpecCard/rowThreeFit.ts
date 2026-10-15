import type { Priority, Resolution, Status } from '../../types/board'
import { RESOLUTION_LABELS, isDroppedResolution } from '../../lib/resolution'

// rowThreeFit decides what the card's status row prints. The row overflowed in
// the worst case — ~303px of the 254px it has — and the overflow clipped the
// verb button, the one control on the row. Rather than let the browser clip
// whatever lands last, the row now budgets itself: every datum gets a compact
// card-only format, and if the total still exceeds the budget the row sheds in a
// fixed order.
//
// The decision is arithmetic, not measured. A DOM measurement would need a
// layout pass per card (95 of them here) and could not be unit-tested; the width
// table below comes from the design source and makes the whole thing a pure
// function. The cost is that the numbers are an approximation of the real glyph
// metrics — deliberately so, with the budget left slightly slack.

/** px per character of the row's 11px mono. */
const CH = 6.6
/** Side of every 11px lucide glyph on the row. */
const GLYPH = 11
/** Gap between a glyph and its own label (`.glyph` / `.priority` use 3px / 5px). */
const GLYPH_GAP = 3
const PRIORITY_GAP = 5
/** The row's flex gap. */
const ROW_GAP = 7
/** The verb button's horizontal padding (`2px 7px`). */
const VERB_PADDING = 14

/** Rendered width of `Urgent` / `High` at 11px/600. */
const PRIORITY_LABEL_WIDTH: Partial<Record<Priority, number>> = { urgent: 38, high: 28 }

/** What the row can occupy: the card minus its border, its status rail and the body padding. */
function budgetFor(cardWidth: number): number {
  return cardWidth - 2 - 2 - 24
}

export interface RowThreeInput {
  priority: Priority
  status: Status
  resolution?: Resolution
  quickWin: boolean
  uat: boolean
  /** The pin prints for a spec's own focus and for focus inherited from its epic. */
  focusVisible: boolean
  sketchCount: number
  estimateMinutes?: number
  tokens: number
  /** Tokens print only with routes recorded, as they already do today. */
  routes: number
  /**
   * The short word the verb button prints (`apply`, `close`, `ship`), not the
   * whole command: nextCommandFor returns `/vector:ship <slug>` and
   * CardVerbButton derives the word from it. `null` when the card has no next
   * move. The transient `copied` state is wider and is not budgeted for — it
   * lasts ~1.5s and the row cannot react to it anyway.
   */
  verb: string | null
  /** 282px: the 290px column minus the 4px padding that keeps focus rings intact. */
  cardWidth?: number
}

export type ShedItem = 'tokens' | 'sketches' | 'estimate'

export interface RowThreeFit {
  showEstimate: boolean
  showSketches: boolean
  showTokens: boolean
  estimateText: string
  tokensText: string
  /** What was dropped, in the order it was dropped. For the tests, and nothing else. */
  shed: ReadonlyArray<ShedItem>
}

/**
 * Card-only estimate: whole hours collapse to `2h`, anything else stays in
 * minutes (`45m`, `90m`, `150m`). The drawer keeps `formatEstimate`'s `90 min` —
 * it has the width for the longer form, the card does not.
 */
export function formatCardEstimate(minutes: number): string {
  if (minutes >= 60 && minutes % 60 === 0) return `${minutes / 60}h`
  return `${minutes}m`
}

/**
 * Card-only token total. Deliberately not `Intl`'s `compact` notation: that
 * collapses 999_999 to `1M`, which would read as a millions-scale number one
 * token short of it. Here the millions step starts at 1_000_000 and 999_999
 * prints `1000k tok`.
 */
export function formatCardTokens(total: number): string {
  if (total < 1_000) return `${total} tok`
  if (total < 10_000) return `${stripTrailingZero((total / 1_000).toFixed(1))}k tok`
  if (total < 1_000_000) return `${Math.round(total / 1_000)}k tok`
  return `${stripTrailingZero((total / 1_000_000).toFixed(1))}M tok`
}

function stripTrailingZero(value: string): string {
  return value.replace(/\.0$/, '')
}

/** A glyph plus its own label, as `.glyph` lays them out. */
function glyphWithLabel(label: string): number {
  return GLYPH + GLYPH_GAP + label.length * CH
}

interface Item {
  key: ShedItem | 'protected'
  width: number
}

export function rowThreeFit(input: RowThreeInput): RowThreeFit {
  const estimateText = input.estimateMinutes ? formatCardEstimate(input.estimateMinutes) : ''
  const tokensText = input.routes > 0 ? formatCardTokens(input.tokens) : ''

  // Order mirrors the rendered row. A datum that does not render has no width
  // and claims no gap, so `n` below counts only what is actually printed.
  const items: Item[] = []
  const protect = (width: number) => {
    if (width > 0) items.push({ key: 'protected', width })
  }

  protect(input.focusVisible ? GLYPH : 0)
  // A closed card prints no flag at all, and only urgent and high ever print.
  const priorityLabel = input.status === 'closed' ? 0 : (PRIORITY_LABEL_WIDTH[input.priority] ?? 0)
  protect(priorityLabel > 0 ? GLYPH + PRIORITY_GAP + priorityLabel : 0)
  protect(
    isDroppedResolution(input.resolution) ? glyphWithLabel(RESOLUTION_LABELS[input.resolution]) : 0,
  )
  protect(input.quickWin ? GLYPH : 0)
  protect(input.status === 'review' && input.uat ? GLYPH : 0)

  if (input.sketchCount > 0) {
    items.push({ key: 'sketches', width: glyphWithLabel(String(input.sketchCount)) })
  }
  if (estimateText) {
    items.push({ key: 'estimate', width: glyphWithLabel(estimateText) })
  }
  if (tokensText) {
    items.push({ key: 'tokens', width: tokensText.length * CH })
  }

  protect(input.verb ? VERB_PADDING + input.verb.length * CH : 0)

  const budget = budgetFor(input.cardWidth ?? 282)
  let kept = items
  const shed: ShedItem[] = []
  // Tokens go first: the drawer shows the same number, and spend is the least
  // actionable datum on the row. The estimate goes last of the three because it
  // is the only one that says anything about the work still ahead.
  for (const candidate of ['tokens', 'sketches', 'estimate'] as const) {
    if (totalWidth(kept) <= budget) break
    if (!kept.some((item) => item.key === candidate)) continue
    kept = kept.filter((item) => item.key !== candidate)
    shed.push(candidate)
  }

  const keeps = (key: ShedItem) => kept.some((item) => item.key === key)
  return {
    showEstimate: keeps('estimate'),
    showSketches: keeps('sketches'),
    showTokens: keeps('tokens'),
    estimateText,
    tokensText,
    shed,
  }
}

function totalWidth(items: readonly Item[]): number {
  const sum = items.reduce((total, item) => total + item.width, 0)
  return sum + ROW_GAP * Math.max(0, items.length - 1)
}
