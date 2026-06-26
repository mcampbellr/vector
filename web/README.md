# Vector — board panel (`web/`)

React + Vite SPA for the Vector kanban board. It is a **read-only projection** of the board
API served by the `vector` binary (`cli/`); the frontend owns no canonical state.

## Develop

Two processes — the Go API and the Vite dev server (which proxies `/api` to it):

```bash
# 1. API + SSE on a fixed port (from the repo root)
vector serve --port 8787

# 2. Vite dev server with hot reload (from web/)
npm install
npm run dev          # http://localhost:5173  → proxies /api to :8787
```

Override the proxy target with `VECTOR_API=http://host:port npm run dev`.

## Build

```bash
npm run build        # tsc -b && vite build → web/dist
```

Serve the built bundle directly from the binary without recompiling:

```bash
vector serve --web-dir web/dist
```

## Embed into the binary (release)

`cli/internal/webui` embeds `dist/` via `embed.FS`. The release pipeline builds this package
and copies the output into the embed dir **before** compiling the binary:

```bash
npm run build
cp -r web/dist/. cli/internal/webui/dist/
go -C cli build -o vector ./cmd/vector
```

A committed placeholder `cli/internal/webui/dist/index.html` keeps the embed (and the Go
build) valid before the first web build. Built `assets/` are gitignored.

## Contract

`src/types/board.ts` mirrors `cli/internal/board/board.go`. Keep them in sync until a typegen
step replaces the hand-mirror.

## Differentiator

`TokenSavingsMeter` rolls up `agent.routed` events from the activity log — how much cheap-agent
routing saved vs the baseline model. This is the commercialization wedge, not a side stat.
