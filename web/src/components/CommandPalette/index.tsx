import { useEffect, useMemo, useRef, useState } from 'react'
import type { KeyboardEvent } from 'react'
import type { Card, Priority } from '../../types/board'
import { matchCards } from './matchCards'
import { PaletteResultRow } from './PaletteResultRow'
import { PalettePriorityFilter } from './PalettePriorityFilter'
import styles from './CommandPalette.module.css'

interface CommandPaletteProps {
  cards: Card[]
  onSelectCard: (card: Card) => void
  onClose: () => void
}

// CommandPalette is the search-and-jump overlay: type to filter the board's
// specs (title, id, priority-as-text, status-as-text), refine with priority
// chips, and Enter/click to open a spec's details drawer from any view. It is
// 100% read-only over the cards already in memory — no fetch, no state writes,
// and it never filters the board behind it. Local state resets via conditional
// mount in App; there is no explicit reset effect. Escape stops propagation so
// the drawer's window listener never sees the same press.
export function CommandPalette({ cards, onSelectCard, onClose }: CommandPaletteProps) {
  const [query, setQuery] = useState('')
  const [priorities, setPriorities] = useState<Priority[]>([])
  const [highlightedIndex, setHighlightedIndex] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)

  const results = useMemo(() => matchCards(cards, query, priorities), [cards, query, priorities])

  // The board updates live over SSE, so the highlighted row can disappear
  // while the palette is open; clamping keeps aria-activedescendant and Enter
  // pointing at a real row.
  const clampedIndex = Math.min(highlightedIndex, Math.max(results.length - 1, 0))
  const highlightedCard: Card | undefined = results[clampedIndex]

  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  function handleSelect(card: Card) {
    onSelectCard(card)
    onClose()
  }

  function togglePriority(priority: Priority) {
    setPriorities((current) =>
      current.includes(priority)
        ? current.filter((item) => item !== priority)
        : [...current, priority],
    )
  }

  function handleKeyDown(event: KeyboardEvent<HTMLDivElement>) {
    if (event.key === 'ArrowDown') {
      event.preventDefault()
      setHighlightedIndex(Math.min(clampedIndex + 1, Math.max(results.length - 1, 0)))
      return
    }
    if (event.key === 'ArrowUp') {
      event.preventDefault()
      setHighlightedIndex(Math.max(clampedIndex - 1, 0))
      return
    }
    if (event.key === 'Enter') {
      event.preventDefault()
      if (highlightedCard) handleSelect(highlightedCard)
      return
    }
    if (event.key === 'Escape') {
      event.stopPropagation()
      onClose()
      return
    }
    if (event.key === 'Tab') {
      // Trivial focus trap: the input is the only focusable control.
      event.preventDefault()
    }
  }

  const counterText =
    results.length === 0
      ? 'No results'
      : results.length === 1
        ? '1 result'
        : `${results.length} results`

  return (
    <div className={styles.overlay} onClick={onClose}>
      <div
        className={styles.palette}
        role="dialog"
        aria-modal="true"
        aria-label="Command palette"
        onClick={(event) => event.stopPropagation()}
        onKeyDown={handleKeyDown}
      >
        <input
          ref={inputRef}
          className={styles.input}
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          role="combobox"
          aria-expanded={results.length > 0}
          aria-controls="command-palette-listbox"
          aria-activedescendant={
            highlightedCard ? `palette-option-${highlightedCard.id}` : undefined
          }
          placeholder="Search specs by title, id, priority or status…"
        />
        <PalettePriorityFilter selected={priorities} onToggle={togglePriority} />
        <p
          className={results.length === 0 ? `${styles.counter} ${styles.empty}` : styles.counter}
          aria-live="polite"
          aria-atomic="true"
        >
          {counterText}
        </p>
        {results.length > 0 && (
          <ul role="listbox" id="command-palette-listbox" className={styles.results}>
            {results.map((card, index) => (
              <PaletteResultRow
                key={card.id}
                id={`palette-option-${card.id}`}
                card={card}
                highlighted={index === clampedIndex}
                onSelect={handleSelect}
              />
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}
