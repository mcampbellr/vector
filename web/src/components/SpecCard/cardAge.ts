import type { Status } from '../../types/board'

// cardAge turns a spec's `updatedAt` into the one datum the card prints about
// time, plus the tone that datum is printed in. It is pure and takes `now` as a
// parameter, like format.ts's helpers: the board resolves a single tick in
// KanbanBoard and passes it down, so no card owns a timer.
//
// Two deliberate departures from the rest of the app's time formatting:
//
//   1. The card's vocabulary is its own. It never prints seconds or minutes
//      (anything under an hour is `now`) and from a week on it uses w/mo/y,
//      which the board header never does. That keeps card freshness and board
//      freshness from reading as the same number.
//   2. Everything floors. `relativeTime` in lib/format.ts rounds, so a 6d 20h
//      spec would print `6d` on the card and "7 days ago" in its own tooltip —
//      two numbers for one datum, side by side. The prose lead below is built
//      from the same floored values, reusing only relativeTime's vocabulary.

/**
 * How loudly the age prints.
 *
 * `quiet` and `neutral` render identically (dim, weight 400). They stay
 * separate because the wording differs: a `neutral` age never says "Stale",
 * since it belongs to a card that cannot rot (see IN_FLIGHT).
 */
export type AgeTone = 'quiet' | 'neutral' | 'stale' | 'very-stale'

export interface CardAge {
  /** What the card prints: `now`, `5h`, `3d`, `1w`, `1mo`, `1y`. */
  text: string
  tone: AgeTone
  /** Bare relative phrase for the tooltip lead and the card's aria-label ("3 days ago"). */
  relative: string
  /** The whole `title` attribute: lead plus the absolute timestamp. */
  tooltip: string
}

/**
 * Only work in flight can rot. `open` is a backlog item waiting its turn and
 * `closed` is finished — both print their age, always neutral.
 */
const IN_FLIGHT: ReadonlySet<Status> = new Set<Status>(['in-progress', 'needs-attention', 'review'])

const HOUR_MS = 3_600_000
const DAY_MS = 86_400_000

interface CardAgeOptions {
  /**
   * Fixes the zone the absolute timestamp is rendered in. Left out in the app
   * (the viewer's own zone is the useful one); set in tests, where the host zone
   * would otherwise decide the expected string.
   */
  timeZone?: string
}

export function cardAge(
  iso: string,
  status: Status,
  now: number,
  options: CardAgeOptions = {},
): CardAge | null {
  if (!Number.isFinite(now)) return null
  const parsed = Date.parse(iso)
  if (Number.isNaN(parsed)) return null
  // Go's zero time.Time serializes as `0001-01-01T00:00:00Z`: parseable, and
  // meaningless. No spec can legitimately carry it — UpdatedAt is written on
  // every transition — so it is read as "no datum" rather than printed as `55y`.
  if (new Date(parsed).getUTCFullYear() <= 1) return null

  // A timestamp from the future means a clock skew, not a spec updated later.
  const elapsed = Math.max(0, now - parsed)
  const tone = toneFor(elapsed, status)
  const relative = relativePhrase(elapsed)

  return {
    text: ageText(elapsed),
    tone,
    relative,
    tooltip: `${leadFor(tone, relative)} · ${absolute(parsed, options.timeZone)}`,
  }
}

function ageText(elapsed: number): string {
  const hours = elapsed / HOUR_MS
  if (hours < 1) return 'now'
  if (hours < 24) return `${Math.floor(hours)}h`
  const days = elapsed / DAY_MS
  if (days < 7) return `${Math.floor(days)}d`
  if (days < 30) return `${Math.floor(days / 7)}w`
  // A month is 30 days flat, so the last days before a year read `12mo`. See the
  // note in the spec: the design's prose said `11mo` while its own logic said
  // floor(days / 30); the logic is what this follows.
  if (days < 365) return `${Math.floor(days / 30)}mo`
  return `${Math.floor(days / 365)}y`
}

function toneFor(elapsed: number, status: Status): AgeTone {
  if (!IN_FLIGHT.has(status)) return 'neutral'
  const days = elapsed / DAY_MS
  if (days >= 30) return 'very-stale'
  if (days >= 7) return 'stale'
  return 'quiet'
}

/**
 * relativeTime's wording, floored. The tooltip is allowed more precision than
 * the card: under an hour it still says "45 sec ago" where the card says `now`.
 */
function relativePhrase(elapsed: number): string {
  const seconds = Math.floor(elapsed / 1000)
  if (seconds < 60) return `${seconds} sec ago`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes} min ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours} hr ago`
  const days = Math.floor(hours / 24)
  return `${days} day${days === 1 ? '' : 's'} ago`
}

function leadFor(tone: AgeTone, relative: string): string {
  if (tone === 'stale') return `Stale — updated ${relative}`
  if (tone === 'very-stale') return `Very stale — updated ${relative}`
  return `Updated ${relative}`
}

function absolute(timestamp: number, timeZone?: string): string {
  return new Intl.DateTimeFormat('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
    ...(timeZone ? { timeZone } : {}),
  }).format(new Date(timestamp))
}
