# AGENTS.md

Onboarding guide for AI agents (and humans) working on **hashdupes**. Read this
first, then consult `PLAN.md` for the full design rationale.

---

## 1. What this project is

hashdupes is a **local-only desktop app** that finds and helps clean up duplicate
**files** and **folders** using content hashing. Backend is Go; UI is Wails v2 +
Svelte 5. It runs strictly on the user's machine — no network, no telemetry.

Duplicate identity = `(size, head_hash)` as a fast bucketing key, **always
confirmed by a lazy byte-by-byte compare** before anything is shown as a true
duplicate or acted upon. Folder duplicates are content-only (names ignored),
computed via a Merkle-style folder hash.

> **Current phase: READ-ONLY.** The UI can scan and view duplicates. Destructive
> actions (trash/move/delete) are intentionally **not wired to the API yet**. The
> `internal/actions` package exists and is tested, but is not bound to the frontend.
> Do not expose write operations without an explicit request.

---

## 2. Tech stack

| Layer | Choice |
|---|---|
| Language | Go **1.25.7** (see `go.mod`; toolchain auto-upgrades) |
| Desktop shell | Wails **v2.12.0** |
| Frontend | Svelte **5** (runes) + Vite **6** + TypeScript |
| Styling | TailwindCSS **3** + shadcn design system (CSS-variable tokens) |
| Icons | `lucide-svelte` (deprecated alias of `@lucide/svelte`; swap pending) |
| Hashing | `github.com/zeebo/blake3` (default) + `crypto/sha256`, behind a `Hasher` interface |
| DB | `modernc.org/sqlite` (pure Go, no cgo) via `database/sql` |
| Migrations | `github.com/pressly/goose/v3`, embedded |
| Concurrency | `golang.org/x/sync` |

---

## 3. Repository layout

```
hashdupes/
├── main.go                 # Wails entry point: DB open, runtime bridge, bind v1 API
├── wails.json              # Wails project config
├── Makefile                # Dev tasks (see §5)
├── PLAN.md                 # Full design doc — source of truth for intent
├── cmd/
│   └── migrate/            # Standalone migration CLI (up/down/status)
├── internal/
│   ├── model/              # Shared domain types (File, Folder, FileDupGroup, ...)
│   ├── hash/               # Pluggable hashers (BLAKE3/SHA-256), head hashing, folder Merkle
│   ├── verify/             # Mandatory byte-by-byte comparison
│   ├── index/              # SQLite store, goose migrations, dup queries
│   │   └── migrations/     # NNNN_*.sql (goose Up/Down)
│   ├── scan/               # Parallel scan pipeline (progress, cancellation, cache reuse)
│   ├── dupes/              # Candidate grouping, confirmation, folder subsumption
│   ├── actions/            # XDG trash + undo log (NOT bound to API in this phase)
│   ├── appdir/             # Per-user paths (DB location)
│   └── api/
│       └── v1/             # Versioned app API + DTOs; Wails-decoupled
└── frontend/
    ├── src/
    │   ├── App.svelte              # Main screen orchestration
    │   ├── main.ts                 # Svelte 5 mount()
    │   ├── app.css                 # Tailwind + shadcn CSS-variable theme
    │   └── lib/
    │       ├── api.ts              # Typed wrappers over window.go / window.runtime
    │       ├── types.ts            # Mirrors of the Go v1 DTOs (keep in sync!)
    │       ├── utils.ts            # cn() (clsx + tailwind-merge)
    │       ├── format.ts           # byte/date formatting helpers
    │       └── components/
    │           ├── ui/             # shadcn-idiom primitives: Button, Badge, Progress, Switch
    │           ├── Results.svelte  # Tabs + summary + group lists
    │           ├── FileGroup.svelte    # Collapsible; lazy verify-on-expand
    │           └── FolderGroup.svelte  # Collapsible folder group
    └── dist/               # Vite build output; embedded by main.go (gitignored)
```

---

## 4. Architecture & data flow

- **API is versioned and Wails-decoupled.** `internal/api/v1.Service` depends on
  two small interfaces — `Emitter` (events) and `DirPicker` (folder dialog) —
  whose Wails-backed implementations live in `main.go` (`bridge`). This keeps the
  API package unit-testable without Wails and allows a future `v2` alongside `v1`.
- **Bound methods** (callable from the frontend as `window.go.v1.Service.*`):
  `ChooseFolder`, `StartScan`, `CancelScan`, `GetReport`, `VerifyGroup`.
- **Scan is async + event-driven.** `StartScan` returns immediately and runs in a
  goroutine; the frontend listens for events:
  - `scan:progress` → `ScanProgress`
  - `scan:done` → `ScanDone` (carries `scanId`)
  - `scan:error` → `ScanError`
- **Lazy verification (key design point).** `GetReport` returns **unverified
  candidate** groups (`verified: false`). Byte-compare happens **on demand** via
  `VerifyGroup` when the user expands a group. Going eager/hybrid later is an
  additive change (call `VerifyGroup` for all groups or in a background job) with
  no change to core logic.
- **Folder subsumption.** File groups whose members all live inside a duplicate
  folder are flagged `withinDupFolder` so the UI can hide them (avoids showing the
  same duplication twice).
- **DTO sync.** `frontend/src/lib/types.ts` mirrors the Go DTOs in
  `internal/api/v1/dto.go`. **When you change a DTO on one side, update the other.**

---

## 5. Setup & commands

### Prerequisites (one-time)
1. **Go 1.25+** and **Node/npm**.
2. **Wails CLI** (pinned to the module version):
   ```bash
   make tools    # go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
   ```
   Ensure `$(go env GOPATH)/bin` is on your `PATH` (e.g. `/home/<you>/go/bin`).
3. **Native libraries (Linux).** Wails needs GTK + WebKit. Verify with `make doctor`.
   On Ubuntu 20.04:
   ```bash
   sudo apt install libgtk-3-dev libwebkit2gtk-4.0-dev
   ```
4. **Frontend deps:** `make deps` (or `wails dev` installs them automatically).

### Everyday commands (Makefile)
| Command | Purpose |
|---|---|
| `make dev` | Run app with hot reload (`wails dev`) |
| `make build` | Clean production build → `build/bin/hashdupes` |
| `make build-debug` | Debug build with devtools |
| `make test` | Run all Go tests |
| `make test-race` | Go tests with `-race` |
| `make vet` / `make fmt` | `go vet` / `go fmt` |
| `make check-frontend` | `svelte-check` type-check |
| `make migrate-status` / `migrate-up` / `migrate-down` | Schema migrations |
| `make doctor` | Check Wails system deps |
| `make clean` | Remove `build/bin` and `frontend/dist` |

### Direct equivalents
```bash
go build ./... && go test ./...           # backend
cd frontend && npm run check && npm run build   # frontend
wails dev                                  # run app
go run ./cmd/migrate -db /path/db status   # migrations against a specific DB
```

The index DB lives at the per-user config dir (e.g. `~/.config/hashdupes/hashdupes.db`)
and migrations run automatically on app startup.

There are `.windsurf/workflows/` for `/dev`, `/build`, and `/migrate`.

---

## 6. Conventions

### Go
- Package-oriented, small focused packages under `internal/`.
- **Do not add or remove comments/docs unless the change requires it.** Match the
  existing thorough doc-comment style on exported symbols.
- Prefer minimal, root-cause fixes over workarounds. Keep changes scoped.
- **Tests are first-class.** Every backend package has passing unit tests; add/extend
  tests with changes, never weaken or delete them without explicit direction. Use
  table-driven tests and `t.TempDir()` for filesystem fixtures.
- Never run destructive shell commands (or bind write actions) without explicit approval.

### Frontend
- **Svelte 5 runes** (`$state`, `$derived`, `$effect`, `$props`) and `{@render}`
  snippets. Use `onclick={...}` (not `on:click`).
- **shadcn design system**: components live in the repo under `lib/components/ui`.
  Style with Tailwind classes + the `cn()` helper and CSS-variable tokens (never
  hard-code theme colors). Dark theme is default (`<html class="dark">`).
- Access Go via `$lib/api.ts` wrappers (which use `window.go` / `window.runtime`),
  not raw globals scattered through components.
- Keep `svelte-check` at **0 errors / 0 warnings**.

---

## 7. Status & roadmap

**Done & tested:** backend core (`model`, `hash`, `verify`, `index`, `scan`,
`dupes`, `actions`), versioned `v1` read-only API, Wails bootstrap, Svelte 5
read-only UI (scan + progress + results with lazy verify), migration CLI, Makefile,
workflows.

**Not yet done (intentionally):**
- No write/trash/move/delete bound to the API (read-only validation phase).
- UI refinement pass is ongoing.
- Richer components (bits-ui-based shadcn Select/Dialog) deferred until the
  write/action UI is built.

See `PLAN.md` §11+ for the phased roadmap.

---

## 8. Gotchas

- **`wails` not found:** the CLI installs to `$(go env GOPATH)/bin`; ensure it's on
  `PATH`, or use `make dev` (calls it by full path).
- **App exits immediately / white flash:** native WebKit libs must be installed;
  the window background is set dark in `main.go` to avoid a flash.
- **Native `<select>` text invisible on dark theme:** WebKit ignores inherited text
  color on selects — set `text-foreground` explicitly and style `<option>`s (see the
  algorithm select in `App.svelte`).
- **`frontend/dist` is gitignored but required by `//go:embed`.** A placeholder
  `index.html` keeps `go build` working; `wails build`/`vite build` produce the real output.
- **`lucide-svelte` is deprecated** in favor of `@lucide/svelte`; a one-line dep swap
  is pending.
- **Keep DTOs in sync** between `internal/api/v1/dto.go` and `frontend/src/lib/types.ts`.
