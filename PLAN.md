# hashdupes — Implementation Plan

A local-only desktop app to find and clean up duplicate **files** and **folders** using a
content-hash Merkle tree. Backend in Go, UI in Wails v2 + Svelte.

---

## 1. Goals & Non-Goals

### Goals
- Scan a chosen root folder, recursively index every file and folder.
- Detect duplicate **files** (identical content) and duplicate **folders** (identical content set).
- Present duplicates in a clear UI so the user can decide what to remove.
- Be fast on large trees via head-only hashing, parallel hashing, and an incremental cache.
- Run strictly locally; no network, no telemetry.

### Non-Goals (initially)
- Cloud/remote storage scanning.
- Fuzzy/near-duplicate detection (e.g. similar images, resized photos).
- Automatic/unattended deletion. The user always confirms actions.

---

## 2. Confirmed Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Hash algorithm | **BLAKE3**, pluggable interface | Faster than SHA-256, internal parallelism, still cryptographically strong. SHA-256 selectable. |
| Folder duplicate semantics | **Content-only** | A folder's signature is derived from the *contents* of its children, ignoring names. Catches renamed copies. |
| MVP cleanup action | **Move to OS Trash/Recycle Bin** | Safe and reversible. Hardlink / permanent-delete / move come later. |
| Persistence | **SQLite via `modernc.org/sqlite`** (pure Go) | No cgo → trivial Wails cross-compilation. Enables incremental cache + persistent scans. |
| File hashing | **Head hash of first `n` bytes for EVERY file** (`n` default 64 KB) | Avoids reading whole large media files. No size-based skipping — every file gets a hash for consistency and queryability. |
| Match correctness | Duplicate identity = `(size, head_hash)`, **always confirmed** by a lazy byte-by-byte compare | Head hash is a fast bucketing key; a mandatory byte-compare (run when a group is viewed/acted on) eliminates false positives. No destructive action ever runs on an unverified group. |
| Empty folders / zero-byte files | **Shown by default**, visually marked; toggle to hide in Settings | All entities are significant to the user; nothing silently filtered. |
| Scan persistence | **Persist scans in DB; resync on rerun** | Reuse prior results, diff against the filesystem, only re-hash what changed. |

---

## 3. Tech Stack

- **Language:** Go 1.22+
- **Desktop shell:** Wails v2
- **Frontend:** Svelte 5 + Vite + TypeScript
- **Styling/UI:** TailwindCSS + a lightweight component set (e.g. shadcn-svelte or skeleton). Lucide icons.
- **Hashing:** `github.com/zeebo/blake3` (or `lukechampine.com/blake3`), behind a `Hasher` interface; `crypto/sha256` as alternate.
- **DB:** `modernc.org/sqlite` (pure-Go driver) via `database/sql`.
- **Trash:** cross-platform "send to trash" lib (e.g. a maintained Go port of trash/`send2trash`), with per-OS fallback.
- **Testing:** standard `testing`, plus `testify` for assertions if desired.

---

## 4. Core Concepts & Algorithms

### 4.1 File record
Each file stores: absolute path, name, parent folder, size, mtime, ctime (where available),
extension/type, `is_empty` flag, `head_hash` (always), optional `full_hash`, and `hashed_at`.

### 4.2 File hashing — head hash for every file
**Every file is hashed**, consistently, by reading only its first `n` bytes (`headBytes`,
default **64 KB**, configurable). If the file is smaller than `n`, the whole file is hashed.
There is **no size-based skipping**: uniform logic, and every file has a populated hash row
that we can query and reuse.

```
headHash(file) = H( read(file, 0, min(n, size)) )
```

- **Duplicate identity = `(size, head_hash)`.** Files of different total size are never duplicates,
  so size participates even though only the head is hashed.
- **Why head-only:** the user has many large media files; reading every byte is wasteful. Capping at
  `n` bytes makes per-file I/O ~constant regardless of file size.
- **Tradeoff — false positives:** two genuinely different files that share the same `size` *and*
  identical first `n` bytes will collide. Rare for real content, but possible (e.g. padded files,
  shared container headers).
- **Mandatory verification (lazy byte-compare):** grouping by `(size, head_hash)` only produces
  *candidate* groups. Before a group is shown as a confirmed duplicate — and **always** before any
  destructive action — its members are confirmed by a **byte-by-byte comparison**. This is:
  - **Exact:** no digest, no collision risk at all.
  - **Cheap on mismatch:** short-circuits at the first differing byte, so a false candidate is
    rejected after reading only a few extra KB rather than the whole file.
  - **Lazy:** verification runs only when a candidate group is expanded/viewed or selected for an
    action — so the full-read cost is paid only for groups the user actually engages with, never for
    the whole collision set up front.
- **Why head hash is still needed:** `(size, head_hash)` is the cheap bucketing key that avoids
  byte-comparing every same-size file pairwise. Verification then confirms within each small bucket.
- **Possible refinement (backlog):** sample head **and** tail (and/or middle) for the bucketing key to
  shrink candidate groups further at negligible extra I/O.

### 4.3 Incremental cache (mitigates "recompute everything")
- Cache key: `(path, size, mtime[, inode])` → `head_hash`. (Byte-compare verification is recomputed lazily at view/action time and isn't cached.)
- On rescan: if a file's `(size, mtime)` matches the cache, reuse the stored hash; no re-read.
- **Persistent scans:** the prior scan's index stays in the DB; a rerun diffs the live filesystem
  against it (added/removed/changed by `size`+`mtime`) and updates only the deltas.
- Only files that changed get re-hashed.
- Folder hashes are recomputed **only along the ancestor chain** of any changed/added/removed child.
- This converts a full rescan from O(all bytes) to O(changed bytes).

### 4.4 Folder Merkle hash (content-only)
For a folder `F`:
```
childTokens = [ "size:head_hash" for each file child ]    # size participates so head-only stays safe
            + [ folderHash(sub)  for each subfolder child ] # recursive
sort(childTokens)                                          # order-independent
folderHash(F) = H( join(childTokens) )
```
Notes / decisions baked in:
- **Content-only:** filenames are intentionally excluded, so renamed copies still match.
- **Size in the token:** each file child contributes `size:head_hash`, not just the head hash, so the
  head-only scheme can't conflate different-sized files inside a folder.
- **Empty folders:** define a canonical empty hash (e.g. `H("")`) so all empty folders share a hash.
  They are **shown by default** and visually marked (not hidden); a Settings toggle can hide them.
- **Skipped entries** (symlinks, ignored files) are excluded from the child set; document this clearly since it
  affects whether two folders are considered equal.

### 4.5 Duplicate detection
- **Candidate files:** `GROUP BY size, head_hash HAVING COUNT(*) > 1`. Each candidate group is then **confirmed by lazy byte-compare** before being treated as a true duplicate group; mismatched members split into separate groups (or drop out).
- **Duplicate folders:** `GROUP BY folder_hash HAVING COUNT(*) > 1`.
- **Zero-byte files** all share `size = 0` and the empty head hash → they group together; shown with a distinct marker, never hidden by default.
- **Subsumption:** when a whole folder is duplicated, its files are *also* duplicates. Present the
  **highest-level duplicate folder** first and collapse the redundant file-level rows beneath it, so the user
  isn't overwhelmed by thousands of child entries. (Mark file dupes whose parent folder is itself a dupe.)

---

## 5. Data Model (SQLite)

```sql
-- One row per scan session (root + settings snapshot).
CREATE TABLE scans (
  id          INTEGER PRIMARY KEY,
  root_path   TEXT NOT NULL,
  started_at  INTEGER NOT NULL,
  finished_at INTEGER,
  algo         TEXT NOT NULL,         -- 'blake3' | 'sha256'
  head_bytes   INTEGER NOT NULL,       -- n bytes hashed per file (e.g. 65536)
  status       TEXT NOT NULL           -- 'running' | 'done' | 'error' | 'canceled'
);

CREATE TABLE files (
  id           INTEGER PRIMARY KEY,
  scan_id      INTEGER NOT NULL REFERENCES scans(id),
  path         TEXT NOT NULL,
  name         TEXT NOT NULL,
  parent_id    INTEGER REFERENCES folders(id),
  size         INTEGER NOT NULL,
  mtime        INTEGER NOT NULL,
  ctime        INTEGER,
  ext          TEXT,
  is_empty     INTEGER NOT NULL DEFAULT 0,  -- 1 if size == 0 (zero-byte marker)
  head_hash    BLOB,                  -- hash of first head_bytes; populated for every file
  verified_grp INTEGER,               -- nullable: id of the byte-compare-confirmed duplicate group
  hashed_at    INTEGER
);

CREATE TABLE folders (
  id           INTEGER PRIMARY KEY,
  scan_id      INTEGER NOT NULL REFERENCES scans(id),
  path         TEXT NOT NULL,
  name         TEXT NOT NULL,
  parent_id    INTEGER REFERENCES folders(id),
  folder_hash  BLOB,
  is_empty     INTEGER NOT NULL DEFAULT 0,  -- 1 if folder has no (non-skipped) children
  file_count   INTEGER,
  total_size   INTEGER
);

-- Persistent, cross-scan cache so unchanged files are never re-hashed.
CREATE TABLE hash_cache (
  path         TEXT NOT NULL,
  size         INTEGER NOT NULL,
  mtime        INTEGER NOT NULL,
  algo         TEXT NOT NULL,
  head_bytes   INTEGER NOT NULL,
  head_hash    BLOB NOT NULL,
  PRIMARY KEY (path, size, mtime, algo, head_bytes)
);

CREATE INDEX idx_files_dup    ON files(size, head_hash);  -- candidate grouping
CREATE INDEX idx_folders_hash ON folders(folder_hash);
```

DB location: OS user-config/data dir (e.g. `~/.local/share/hashdupes/` on Linux) so it survives between runs.

---

## 6. Backend Architecture (Go)

Suggested package layout:
```
hashdupes/
├── main.go                 # Wails bootstrap
├── app.go                  # App struct: methods bound to the frontend (Wails bindings)
├── wails.json
├── go.mod
├── internal/
│   ├── scan/               # walk, worker pool, progress, cancellation
│   │   ├── walker.go
│   │   ├── pipeline.go
│   │   └── progress.go
│   ├── hash/               # Hasher interface + blake3/sha256 impls + file/folder hashing
│   │   ├── hasher.go
│   │   ├── file.go
│   │   └── folder.go
│   ├── index/              # SQLite store, schema, cache, dedup queries
│   │   ├── store.go
│   │   ├── schema.go
│   │   └── queries.go
│   ├── dupes/              # duplicate grouping + subsumption logic
│   ├── actions/            # trash / (later) hardlink / delete, with dry-run + undo log
│   └── model/              # shared DTOs returned to the frontend
└── frontend/               # Svelte + Vite
```

### Concurrency model
- **Walker goroutine** produces file entries onto a channel.
- **Bounded worker pool** (size ≈ NumCPU, configurable) consumes and hashes candidate files.
- **Folder hashing** runs as a post-pass once all child hashes are known (bottom-up / topological order).
- **Progress events** (`files scanned`, `bytes hashed`, `current path`, `ETA`) emitted to the UI via
  Wails runtime events, throttled (e.g. every 100ms) to avoid flooding.
- **Cancellation** via `context.Context`; UI "Stop" cancels mid-scan and persists partial results.

---

## 7. Frontend (Wails + Svelte)

### Bound Go methods (API surface)
```
ChooseFolder() (path string)                      // native dir picker
StartScan(opts ScanOptions) (scanId int)
CancelScan(scanId int)
GetDuplicateFiles(scanId) []FileDupGroup
GetDuplicateFolders(scanId) []FolderDupGroup
GetTreeNode(scanId, folderId) []TreeEntry         // lazy tree expansion
TrashPaths(paths []string) ActionResult           // MVP cleanup
GetSettings() / SaveSettings(...)
```
Events: `scan:progress`, `scan:done`, `scan:error`, `action:done`.

### Screens
1. **Start / Scan** — pick root, choose options (algo, ignore rules, follow symlinks), big Start button, live progress (counts, bytes, current path, ETA, cancel).
2. **Results — Duplicates view** (primary):
   - Two tabs/sections: **Duplicate Folders** and **Duplicate Files**.
   - Each group shows the copies, sizes, paths, and **reclaimable space** (size × (copies−1)).
   - Folder dupes shown first; file dupes that live inside a duplicate folder are collapsed/flagged.
   - **Empty folders** and **zero-byte files** are shown by default, each with a distinct icon/badge so they're easy to spot; both can be toggled off in Settings.
   - Per group: pick which copy to **keep**, select the rest → **Move to Trash**.
   - Bulk helpers: "keep newest / keep in path X / keep shortest path".
3. **Tree view** (secondary) — full folder tree with badges marking duplicated nodes; lazy-loaded.
4. **Settings** — hash algo (BLAKE3/SHA-256), **head-hash byte length** (`n`, default 64 KB), worker count, ignore globs, include/exclude hidden, follow-symlinks, **show/hide empty folders**, **show/hide zero-byte files** (both shown by default). _Match verification is always on (byte-compare) and intentionally has no toggle._

### UX safety rails
- Always show total **reclaimable space** before any action.
- Default action = **Trash** (reversible). Confirmation dialog summarizing exactly what will be moved.
- Maintain an **undo log** of the last action (source → trash) for an in-app "Undo".
- Never allow deleting *every* copy in a group (force at least one "keep").
- **No destructive action ever runs on an unverified group:** selecting a group for trashing triggers its byte-compare confirmation first; any member that fails to match is excluded from the action.

---

## 8. Cross-Platform Considerations
- **Timestamps:** Go exposes `mtime` portably; `ctime`/birth-time require `syscall`/`os.Sys()` per OS. Treat `ctime` as best-effort/nullable.
- **Path handling:** use `filepath` throughout; store absolute, cleaned paths. Be mindful of case-insensitive filesystems (Windows/macOS).
- **Symlinks:** default **do not follow** (avoid cycles + double counting); make it an option. Detect cycles via visited inode/device set if following is enabled.
- **Permissions:** record and skip unreadable files/folders gracefully; surface a "skipped" report.
- **Trash:** verify the chosen library covers Windows Recycle Bin, macOS Trash, and Linux XDG trash; provide a fallback.

---

## 9. Edge Cases to Handle
- Zero-byte files (all share `size 0` + empty head hash → grouped together; **shown by default** with a distinct marker, with a Settings toggle to hide).
- Head-hash false positives (same `size` + same first `n` bytes, different tails) — eliminated by the mandatory lazy byte-compare before a group is confirmed or acted upon.
- Empty folders (canonical empty hash; shown by default with a marker).
- Very large files (stream-hash; never load fully into memory).
- Files changing mid-scan (record mtime at hash time; flag if changed).
- Hardlinks / same inode counted once (detect via `(device, inode)`).
- Deeply nested trees / huge file counts (lazy UI loading, batched DB inserts via transactions).
- Mixed empty folders and skipped entries affecting folder-hash equality (document + make configurable).
- Permission-denied and I/O errors (collect, continue, report).

---

## 10. Phased Roadmap

### Phase 0 — Scaffold
- `wails init` (Svelte + TS template), Tailwind, repo layout above, `go.mod`.
- Hello-world IPC: a bound Go method returning data to a Svelte page.

### Phase 1 — Backend hashing core (no UI)
- `Hasher` interface + BLAKE3 & SHA-256 impls.
- File walker + parallel **head hashing** (first `n` bytes for every file) + optional full-verify pass.
- Folder Merkle hashing (content-only, `size:head_hash` child tokens), bottom-up.
- Unit tests on a fixture tree with known duplicates (incl. empty folders, zero-byte files, head-collision case).

### Phase 2 — Persistence & dedup
- SQLite store + schema + batched inserts.
- `hash_cache` for incremental rescans.
- Duplicate-file and duplicate-folder queries + subsumption/collapse logic.

### Phase 3 — Scan UI
- Folder picker, scan options, **live progress + cancel** via Wails events.
- Results: duplicate folders + files, reclaimable-space totals.

### Phase 4 — Cleanup (Trash)
- Multi-select, keep-one enforcement, confirmation dialog, move-to-trash, undo log.

### Phase 5 — Tree view & polish
- Lazy tree with duplicate badges, settings screen (incl. empty/zero-byte visibility toggles, head-byte length, full-verify), ignore rules, packaging/build for target OSes.

### Later / Backlog
- Hardlink-dedup and permanent-delete actions.
- Saved scans / scan history & diff between scans.
- Near-duplicate (image/media) detection.
- Export report (CSV/JSON).

---

## 11. Resolved Decisions (was: Open Questions)
- **Empty folders & zero-byte files:** shown by default, visually marked; hide-toggles in Settings. ✅
- **Head hashing:** hash first `n` bytes of *every* file (default `n` = 64 KB), no size-based skipping. ✅
- **No minimum file-size filter:** all files, including empty ones, are significant. ✅
- **Scan persistence:** scans are stored in the DB and resynced (diffed) against the filesystem on rerun. ✅

- **Match verification:** always-on, mandatory **byte-by-byte compare** of candidate groups (no toggle); runs lazily when a group is viewed or selected for action, and always before any destructive action. ✅

### Remaining minor questions
- Head-byte default: 64 KB confirmed as starting point — revisit if needed.
- Head **+ tail** sampling for the bucketing key: adopt now or keep as backlog refinement?
