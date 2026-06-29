# Tasks — add-resizable-markdown-preview-modal

## 1. Sizing model + persistence helpers (web)

- [x] 1.1 In `web/src/components/SpecDetailsDrawer/FilePreviewModal.tsx`, add
  `interface ModalSize { width: number; height: number }` and bound constants
  `DEFAULT_WIDTH = 720`, `MAX_WIDTH_RATIO = 0.95`, `MAX_HEIGHT_RATIO = 0.95`. Capture
  `DEFAULT_HEIGHT = window.innerHeight * 0.86` once at state init (not per render).
- [x] 1.2 Module-level pure helper `loadPersistedSize(): ModalSize | null` — read
  `localStorage['vector:file-preview-modal:size']` in `try/catch`, `JSON.parse`, verify
  `width`/`height` are finite `number`s, re-clamp to the current viewport
  (`Math.max(DEFAULT_WIDTH, Math.min(v, window.innerWidth * MAX_WIDTH_RATIO))`, analogous for
  height). Return `null` on any error/invalid shape.
- [x] 1.3 Module-level pure helper `savePersistedSize(size: ModalSize): void` —
  `JSON.stringify` + `localStorage.setItem` under the same key, silent `try/catch`.
- [x] 1.4 Seed `useState<ModalSize>(() => loadPersistedSize() ?? { width: DEFAULT_WIDTH, height: DEFAULT_HEIGHT })`.

## 2. Drag interaction (web)

- [x] 2.1 Three `mousedown` handlers keyed by axis (`'right' | 'bottom' | 'corner'`): record
  `startX/startY/startWidth/startHeight` and the active axis, `e.preventDefault()` to block
  text selection, register `onMouseMove` + `onMouseUp` on `document`, pin the active cursor on
  `document.body`.
- [x] 2.2 `onMouseMove`: inside a `requestAnimationFrame`, compute the delta, clamp with
  `Math.max(floor, Math.min(value, ceiling))` (floor = default captured on mount; ceiling
  recomputed from `window.innerWidth/innerHeight * 0.95` each event), update only the axis the
  active handle owns. Hold the `frameId` in a `useRef` and `cancelAnimationFrame` it if needed.
- [x] 2.3 `onMouseUp`: remove the `document` listeners, restore `document.body` cursor, call
  `savePersistedSize(size)`.
- [x] 2.4 `useEffect` cleanup that removes any lingering `document` `mousemove`/`mouseup`
  listeners (and cancels a pending `frameId`) if the component unmounts mid-drag.

## 3. Render + styles (web)

- [x] 3.1 Apply `style={{ width: `${size.width}px`, height: `${size.height}px` }}` to
  `.modalPanel`; render the three handles inline, or extract a sibling `ResizeHandle.tsx`
  (promoting `FilePreviewModal.tsx` → `FilePreviewModal/index.tsx`) if the markup justifies it.
  Each handle: `role="separator"`, `tabIndex={0}`, axis `aria-label`
  (`"Redimensionar ancho"` / `"Redimensionar alto"` / `"Redimensionar modal"`),
  `aria-orientation` for the two edges (none for the corner).
- [x] 3.2 In `web/src/components/SpecDetailsDrawer/SpecDetailsDrawer.module.css`: add
  `position: relative` to `.modalPanel`; remove the fixed `width` (keep `max-width: 95vw`
  fallback) and the `max-height: 86vh`.
- [x] 3.3 Add `.resizeHandleRight` / `.resizeHandleBottom` / `.resizeHandleCorner` (absolute
  position, `col-resize` / `row-resize` / `se-resize` cursor, `--color-border`-derived fill,
  `z-index: 1`). No new CSS variables; do not touch `.modalOverlay` / `.modalBody` /
  `.modalHeader` or the z-index 60 overlay.
- [x] 3.4 Add `@media (max-width: 640px), (pointer: coarse) { … { display: none } }` hiding the
  three handles.

## 4. Verification

- [x] 4.1 From `web/`: `npm run typecheck` — green (no `any`).
- [x] 4.2 From `web/`: `npm run build` — green.
- [x] 4.3 Vitest (`web/package.json` declares `^4.1.9`): unit-test `loadPersistedSize` (valid →
  size; malformed JSON → `null`; out-of-viewport → re-clamped; absent → `null`) and the clamp
  (below floor → floor; above ceiling → ceiling). If a runtime issue blocks it, record as debt.
- [x] 4.4 Manual checks from the success criteria: right/bottom/corner drag grows the correct
  axis in real time; cannot shrink below default; cannot exceed ~95vw/95vh; size restored on
  reopen and re-clamped when stored value exceeds the viewport; no crash without `localStorage`;
  handles hidden ≤640px / coarse pointer; Escape / overlay / X close and focus restore unchanged;
  `.modalBody` scroll and lazy `MarkdownView` unchanged.
