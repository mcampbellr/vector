# Design — add-resizable-markdown-preview-modal

## Context

`FilePreviewModal` (`web/src/components/SpecDetailsDrawer/`) is the in-board markdown reader
introduced by `add-spec-file-viewer`. Its `.modalPanel` is fixed at `min(720px, 94vw)` wide
with `max-height: 86vh` (`SpecDetailsDrawer.module.css`); it closes on Escape / overlay click
/ X, restores focus to the previously focused element, and renders content through a lazily
code-split `MarkdownView`. The panel has no resize affordance and no notion of a remembered
size. This change adds drag-to-resize and a persisted size, scoped strictly to this component
plus its CSS module — no API, state-JSON, or backend surface is touched (sizing is a pure
client preference).

## Goals / Non-Goals

**Goals:**
- Let the user grow the modal from the right edge (width), bottom edge (height), and
  bottom-right corner (both), in real time, with no perceptible lag.
- Clamp the size to a floor (the default — the modal never shrinks below it) and a ~95vw/95vh
  ceiling recomputed per frame so the panel never escapes the viewport.
- Persist one global size in `localStorage` and restore it (re-clamped) on reopen, degrading
  silently when storage is unavailable.
- Keep the renderer, open/close, focus management, and `.modalBody` scroll untouched.

**Non-Goals:**
- Resizing from the left or top edge; CSS `resize: both`; any external drag library.
- Touch / `pointer: coarse` support (desktop-only this phase); keyboard arrow-key resizing
  (deferred enhancement); per-file sizing.
- Any change to the close behavior, focus management, `MarkdownView` code-splitting, or
  `.modalBody` scroll; any backend, API, or state-JSON change.

## Decisions

- **State + DOM-native events, no library.** The size lives in `useState<ModalSize>`
  (`interface ModalSize { width: number; height: number }`). `mousedown` on a handle records
  `startX/startY/startWidth/startHeight` and the active axis (`'right' | 'bottom' | 'corner'`),
  registers `mousemove`/`mouseup` on `document` (to capture motion outside the panel), and
  calls `preventDefault()` to suppress text selection. `mouseup` removes the listeners and
  persists. Rationale: zero new deps, full control over bounds and accessibility.
- **`requestAnimationFrame` throttle.** Each `mousemove` schedules the state update inside an
  rAF; the `frameId` is held in a `useRef` and `cancelAnimationFrame`d if `mouseup` lands
  before the next frame. State updates at most once per frame (~60fps), avoiding excess
  re-renders without a throttle utility.
- **Floor = default, captured on mount; ceiling recomputed per frame.** `DEFAULT_WIDTH = 720`,
  `DEFAULT_HEIGHT = window.innerHeight * 0.86` (read once when the state initializes, constant
  per open session). Per frame, `Math.max(floor, Math.min(delta, ceiling))` with ceiling
  `window.innerWidth * 0.95` / `window.innerHeight * 0.95`. Only the axis owned by the active
  handle changes. Rationale: preserve a minimum usable size; never let the modal leave the
  viewport even if the browser window is resized mid-drag.
- **`localStorage` persistence, global key, defensive I/O.** Pure module-level helpers (outside
  the component, to keep them unit-testable): `loadPersistedSize(): ModalSize | null` reads
  `vector:file-preview-modal:size`, `JSON.parse`s, validates `width`/`height` are finite
  numbers, re-clamps to the current viewport, and returns `null` on any failure;
  `savePersistedSize(size)` `JSON.stringify`s + `setItem`, both wrapped in silent `try/catch`.
  `useState` is seeded with `() => loadPersistedSize() ?? { width: DEFAULT_WIDTH, height: DEFAULT_HEIGHT }`.
  Persist runs only on `mouseup`, never per frame. Rationale: a size preference is UX, not
  content (so one global size); storage-less / private-mode contexts must not crash.
- **Inline style is the single source of truth for size.** `.modalPanel` gets
  `position: relative` (so absolutely-positioned handles anchor to it) and an inline
  `style={{ width: `${size.width}px`, height: `${size.height}px` }}`; the CSS drops the fixed
  `width` (keeps `max-width: 95vw` as a fallback) and the `max-height: 86vh`. Handles are
  `position: absolute` children at `z-index: 1` (over `.modalBody`, irrelevant to the overlay's
  z-index 60 in a separate stacking context), tinted from `--color-border`.
- **Desktop-only via CSS, accessible handles.** `@media (max-width: 640px), (pointer: coarse)`
  sets the three handle classes to `display: none`; the modal keeps responsive sizing there.
  Each handle: `role="separator"`, `tabIndex={0}`, `aria-label` in Spanish
  (`"Redimensionar ancho"` / `"Redimensionar alto"` / `"Redimensionar modal"`),
  `aria-orientation` `vertical`/`horizontal` for the edges (none for the corner), and the
  matching cursor. During drag the active cursor is pinned on `document.body` and restored on
  `mouseup`.
- **`ResizeHandle` extraction is conditional.** If the handle markup grows enough to warrant
  it, promote `FilePreviewModal.tsx` to `FilePreviewModal/index.tsx` with a sibling
  `ResizeHandle.tsx` (props `{ axis, onMouseDown }`, no own state, no size logic), per the
  one-component-per-file rule. If the handles stay trivially inline, keep the single file.

## Open questions (carried from the spec)

- **Exact handle thickness / hit area** → start at ~6px visible with a `::after`/padding hit
  area of ~8–12px; confirm or set as a project convention.
- **Vitest coverage in `web/`** → `web/package.json` declares Vitest `^4.1.9` (`test: vitest run`),
  so add unit tests for `loadPersistedSize` and the clamp; if a runtime issue blocks it, record
  as debt.
- **Inline style in small viewports** → with handles hidden the inline `width`/`height` still
  applies; optionally add `max-width: min(720px, 94vw); height: auto` to `.modalPanel` inside
  the media query to restore the original responsive sizing. Confirm if desired.
- **`mouseup` outside the window** → some browsers don't fire `mouseup` on `document` when the
  button is released outside the tab, leaving a hung drag; optionally register `mouseleave` /
  `blur` on `window` as a cancellation fallback, or accept the edge case.
- **Keyboard arrow-key resizing** → deferred enhancement; handles are already focusable for it.
