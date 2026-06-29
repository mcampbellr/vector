# Resizable markdown preview modal (right / bottom / corner handles)

## Why

The board's markdown preview modal (`FilePreviewModal`) opens at a fixed size
(`min(720px, 94vw)` wide, `max-height: 86vh`). Reading a long `spec.md`, `proposal.md`, or
`tasks.md` forces the user to scroll inside a cramped panel with no way to widen or lengthen
it. There is no in-board way to give the reading surface more room, and any size the user
might prefer is forgotten on every reopen.

## What changes

- **Three drag handles on the preview panel (web)** — `FilePreviewModal` gains a right-edge
  handle (adjusts width only), a bottom-edge handle (adjusts height only), and a
  bottom-right corner handle (adjusts both). Dragging grows the modal in real time. Built
  exclusively on native DOM events (`mousedown` on the handle → `mousemove`/`mouseup` on
  `document`), throttled with `requestAnimationFrame`. No drag library, no CSS `resize`.
- **Bounded, persisted sizing (web)** — the modal size lives in
  `useState<ModalSize>`. Floor = the default size (720px wide; `window.innerHeight * 0.86`
  tall, captured on mount) — the modal can only grow, never shrink below it. Ceiling ~95vw /
  ~95vh, recomputed every drag frame so the panel never escapes the viewport. The chosen size
  is persisted to `localStorage` under `vector:file-preview-modal:size` (one global size, not
  per-file) on `mouseup`, and restored — with a re-clamp to the current viewport — on reopen.
  All `localStorage` access is wrapped in `try/catch`; storage-less environments fall back to
  in-memory sizing without error.
- **Desktop-only, accessible handles (web)** — handles are hidden via
  `@media (max-width: 640px), (pointer: coarse)`; the modal keeps its original responsive
  sizing there. Each handle carries `role="separator"`, `tabIndex={0}`, a Spanish
  `aria-label` per axis, and the matching resize cursor (`col-resize` / `row-resize` /
  `se-resize`).

## Capabilities

### Modified Capabilities
- `spec-file-viewer`: the preview modal becomes resizable from its right edge, bottom edge,
  and bottom-right corner, with the size clamped to a floor (the default) and a ~95vw/95vh
  ceiling and persisted globally in `localStorage`. Additive — the existing open/close
  (Escape / overlay / X), focus management, lazy `MarkdownView`, and `.modalBody` scroll are
  unchanged.

## Impact

- `web/src/components/SpecDetailsDrawer/FilePreviewModal.tsx` (or promoted to a
  `FilePreviewModal/` folder with a sibling `ResizeHandle.tsx` if the handle markup is
  extracted): `interface ModalSize`, drag state + three `mousedown` handlers, pure
  `loadPersistedSize` / `savePersistedSize` helpers, `requestAnimationFrame` throttle with a
  `frameId` ref, `useEffect` cleanup for orphaned `document` listeners, and an inline
  `style={{ width, height }}` on `.modalPanel`.
- `web/src/components/SpecDetailsDrawer/SpecDetailsDrawer.module.css`: `position: relative` on
  `.modalPanel`, drop the fixed `width` (keep `max-width: 95vw` as a safety fallback) and the
  `max-height: 86vh`, add `.resizeHandleRight` / `.resizeHandleBottom` / `.resizeHandleCorner`
  (absolute position, cursor, `--color-border`-derived fill), and the small-viewport /
  coarse-pointer media query that hides them.
- **No new dependency. No Go, API, state-JSON, or backend change** — sizing is a pure
  client-side UI preference; `localStorage` holds only `{ width: number, height: number }`.
  The board stays read-only.

Authored spec: `.vector/specs/add-resizable-markdown-preview-modal/spec.md`.
