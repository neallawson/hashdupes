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

DB location: OS user-config/data dir so it survives between runs.

> ⚠️ **Doc/code discrepancy.** `appdir.DefaultDBPath()` currently uses
> `os.UserConfigDir()`, so the DB actually lands at `~/.config/hashdupes/hashdupes.db`
> on Linux, not `~/.local/share/hashdupes/` as previously documented here. The index
> is a regenerable cache rather than configuration, so XDG argues for
> `~/.local/share` (or `~/.cache`); it also determines whether a user's backup tool
> sweeps the DB up. **To decide:** move the DB and migrate existing installs, or
> accept the config dir and correct the docs. See §12.

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
  → Superseded by the tiered-hashing proposal in **§12.2**; decide there instead.

> ⚠️ Two of the decisions above (**head hashing for every file**, and
> **byte-compare as the confirmation mechanism**) are revisited in **§12**, which
> is open and should be settled before the write/action phase.

---

## 12. Open Decision — Hashing Strategy & Verification Cost

> **Status: OPEN. Address soon** — ideally before Phase 4 (Cleanup), because the
> outcome changes what "confirmed" means and what a destructive action must
> re-check. No code has changed for this section yet.

### 12.1 Why this is open

The current scheme is: head-hash every file → bucket by `(size, head_hash)` →
confirm each bucket with a **byte-by-byte compare**. The byte-compare exists
because a 64 KB head hash genuinely can collide (see §4.2).

The observation driving this section: **hashing a file in full removes the logical
need for byte-comparison at all**, because a full 256-bit digest plus size is
sufficient to call two files identical. That is true, and it is what most
mainstream dedupe tools (`rmlint`, `jdupes`) rely on. But it is not free, and the
naive version — full-hash every file — is the single most expensive option
available. The three decisions below separate the parts of that trade so they can
be settled independently.

### 12.2 Decision A — Adopt tiered hashing?

**Proposal:** replace "head hash everything, then byte-compare" with three stages,
each of which only runs on what survived the previous one:

| Stage | Key | I/O cost |
|---|---|---|
| 0 | `size` alone, from `stat` | **zero bytes read** |
| 1 | head hash, only within size buckets holding >1 file | `min(n, size)` per surviving file |
| 2 | full hash, only within `(size, head_hash)` buckets | full read, once per surviving file |

Compare digests at stage 2; no byte-compare needed for display.

**Why it wins.** It keeps the cheap first pass (a file with a unique size is
provably not a duplicate and is never opened) *and* eliminates byte-compare, and
unlike a byte-compare result a full hash is **cacheable and reusable across
scans** (§4.3 explicitly does not cache verification). So repeat cost drops from
"paid again on every view" to "paid once, ever".

**Why the naive alternative loses.** Full-hashing *every* file inverts the I/O
profile that §4.2 was written to avoid: for ~100k files totalling ~500 GB,
head-hashing reads ~6 GB while hashing everything reads ~500 GB. Stage 0 is what
makes tiering viable — it means the full reads in stage 2 land only on files that
already share both a size and a head hash with something else.

**Note:** with a full hash, `size` stops being needed for *correctness* (different
sizes would essentially never collide) and becomes purely a cheap pre-filter. The
`size:head_hash` folder-hash token (§4.4) would become `size:full_hash`.

**To decide:** adopt tiering, or keep head-hash + byte-compare?

### 12.3 Decision B — Does every file still need a head hash?

§4.2 and §11 decided to hash the head of **every** file with "no size-based
skipping," for *consistency and queryability* — not for correctness.

That decision is now worth re-examining on its own merits, separately from
Decision A. As written it spends I/O on every file whose size is unique in the
whole tree, and such a file **cannot** be a duplicate of anything, so the read
buys nothing for dedup. It buys only the ability to query a populated
`files.head_hash` for every row.

**The cost is worse than "64 KB per file" suggests.** `hash.effectiveHeadLen`
returns `size` when `size < headBytes`, so head-hashing a file smaller than 64 KiB
reads it **in full**. Therefore:

| Tree shape | Fraction of all bytes read by eager head hashing |
|---|---|
| Photo/video library (multi-MB files) | ~1% |
| Source tree, mail, documents (most files < 64 KiB) | **~100%** |

For small-file collections, "head-hash everything" degenerates into "read the whole
tree" — precisely the outcome §4.2 exists to prevent. Size-gating eliminates that
case entirely, and it is where the current design is weakest.

**What eager head hashing actually buys is narrower than it appears.** It is *not*
needed for cross-scan or cross-root duplicate queries: `size` is stored for every
file at zero I/O cost, so cross-scan candidate discovery works from stored data
alone, and candidates are resolvable by hashing on demand because deduping live
trees presupposes the files still exist (see §12.7).

The only capability it uniquely buys is content identity for files that **no longer
exist**. And for that purpose a *head* hash is the wrong artifact: it can never
**confirm** identity, because confirmation requires returning to the file to
byte-compare or full-hash it, which a deleted file forbids. A stored head hash
yields an unconfirmable candidate match — exactly the state §7 forbids acting on.
If a durable manifest of past contents is wanted, that argues for storing **full**
hashes, which is a separate and far more expensive decision that should be argued
on its own merits rather than inherited as a side effect of §4.2.

**Note — lazy hashing is self-warming.** `hash_cache` has no `scan_id`; it is
keyed `(path, size, mtime, algo, head_bytes)` and is genuinely cross-scan. Every
file that *does* get hashed stays cached indefinitely and is reusable by any future
scan of any root, so the hashed set organically grows to cover everything that ever
mattered while never paying for files that never collided.

**Blocker:** the folder Merkle hash (§4.4) consumes a head hash for every file
child, so this decision cannot be taken in isolation — see **§12.8**.

**To decide:** is per-row hash queryability worth a full read of every small file
in the tree? Options: keep as-is; make it a setting (e.g. an opt-in "full index /
manifest" mode, off by default, ideally with full hashes since that is what the
archival use case needs); or drop to size-gated hashing (stage 0 above) and accept
that `head_hash` is `NULL` for unique-size files.

`index.CandidateFileGroups` already filters `WHERE head_hash IS NOT NULL`, so the
duplicate-grouping query needs **no change** under size-gating.

### 12.4 Decision C — Keep byte-compare as the pre-action freshness gate

**Proposal: yes, keep it, regardless of how A and B land.** Even under tiered
hashing, retain a byte-compare immediately before any destructive action (the
existing `dupes.VerifyForAction`).

The reason is **not** collision risk — it is **staleness**. A cached hash can
describe a file as it was, not as it is: `mtime` granularity is coarse (one second
in places), and a file can be modified with its `mtime` deliberately preserved, so
the `(path, size, mtime)` cache key in §4.3 can miss a real change. Byte-comparing
at action time reads what is actually on disk *now*.

This does not close the race (a file can still change between the compare and the
trash call) but it narrows it substantially, and the cost is negligible because it
runs only on the handful of files a user has chosen to delete. It preserves the
§7 safety rail — *no destructive action ever runs on an unverified group* — with
its original meaning intact.

### 12.5 Defect to fix independently — representative re-read

`verify.PartitionByContent` compares each new member against a group
representative via `verify.EqualFiles`, which re-opens and re-reads **both** files
every call. So the representative is read once per member:

| Identical copies (N) | byte-compare reads | one-pass / full-hash reads |
|---|---|---|
| 2 | 2 × size | 2 × size |
| 3 | 4 × size | 3 × size |
| 10 | 18 × size | 10 × size |

That is `2(N-1) × size` versus `N × size`. The OS page cache hides this for files
that fit in RAM, but **not** for the large media files this tool targets — exactly
the case §4.2 was optimised for. Consequence: expanding a large multi-copy group
is roughly twice as slow as it needs to be.

**Fix (independent of A/B/C):** stream all members of a candidate group in a
single parallel pass, reading each file exactly once. This is worth doing even if
byte-compare is retained everywhere.

### 12.6 For the record — why dropping byte-compare is safe on collision grounds

- A full BLAKE3 or SHA-256 digest is 256 bits, so the birthday bound on an
  accidental collision is ~2⁻¹²⁸ — orders of magnitude below the probability of
  silent RAM/disk corruption *during the byte-compare itself*.
- Both algorithms are collision-resistant against deliberate attack, so a crafted
  pair is also infeasible. (This would **not** hold for MD5 or SHA-1; do not add
  either as a selectable algorithm for identity purposes.)
- The one genuine asymmetry favouring byte-compare: it short-circuits at the first
  differing byte, so a false head-hash candidate is rejected after ~128 KB,
  whereas a full hash must read both files to completion to discover they differ.
  Relevant only in the rare collision case.

The exactness argument therefore does not decide this question; cost does.

### 12.7 Cross-scan tooling — history, diff, cross-root dedup

Three features are wanted, and all three are **independent of Decision B**. They
should not be used to justify eager head hashing.

**Current state: the data is already there; nothing reads it back.** No query or
binding touches the `scans` table. Meanwhile every scan persists a full snapshot
(`files` / `folders`, `ON DELETE CASCADE` on `scan_id`) plus `root_path`, `algo`,
`head_bytes`, `status`, `started_at`, `finished_at`. So there is history, but no
history *feature*. Because `root_path` is stored, "all scans" and "scans of this
folder" are one `WHERE` clause apart — both come for free.

Observed in a real DB, illustrating why this is worth surfacing:

```
#2 /home/neal/farside  blake3/65536  done  2026-09-20 12:46:50  14.86s
#3 /home/neal/farside  blake3/65536  done  2026-09-20 12:55:33   0.13s
```

A rescan of the same tree ran **114× faster** off `hash_cache`. That is §4.3
working exactly as designed and currently invisible to the user.

**What each feature needs:**

| Feature | Requires | Hashes needed? |
|---|---|---|
| Scan history / per-root history | `scans` table only | none |
| Diff two scans of one tree | `path`, `size`, `mtime` join on `scan_id` | none |
| Detect moves/renames | `(size, mtime)` at a new path, or exactly `(device, inode)` | none |
| Cross-root dedup | join on `size`, then hash candidates on demand | on demand only |

Cross-root dedup ("is anything in my laptop tree also in my backup tree?") is the
one that looks like it needs eager hashing and does not: tier 0 is `size`, which is
stored for every file at zero I/O cost, so candidate discovery is pure SQL over
existing data. Tiering extends across scans exactly as it does within one.

**Two things to plan for:**
- **Indexes are scan-scoped.** `idx_files_dup ON files(scan_id, size, head_hash)`
  leads with `scan_id` and will not serve a cross-scan join on `size`. Needs a
  small migration.
- **Retention is unsolved.** Full per-scan snapshots with no pruning policy will
  bloat the DB on repeated scans of large trees. A history feature needs a
  retention rule alongside it (keep last N, or keep only user-pinned scans).

**Recommendation:** build all three; they are high value and mostly SQL over data
already being stored. But settle them independently of §12.2/§12.3.

### 12.8 Folder Merkle hash under tiered hashing

**Question: does size-gating file hashing break the folder Merkle hash?**
Naively yes; under tiering, no — and tiering also fixes an existing gap.

`hash.Child.token()` renders a file child as `size:hexhash`, so computing *any*
folder hash requires a head hash for *every* file beneath it, recursively. If
unique-size files were left unhashed, their token would have to degrade to `size:`.

The degraded token does **not** cause false folder duplicates: for two folders to
collide they would each need a `5000:` token, i.e. each contains a file of size
5000 — but then size 5000 is not unique and both files would have been hashed. The
premise is self-defeating.

The real damage is that `folder_hash` stops being **intrinsic** and becomes a
function of the whole scan's size histogram:
- **Cross-scan comparison breaks** — the same folder scanned under a narrow root vs
  a wide root yields different hashes.
- **§4.3's invalidation rule becomes silently wrong** — adding one file *anywhere*
  can flip a distant, untouched file from unique-size to shared-size, changing its
  token and its ancestors' hashes, while that subtree is not on the ancestor chain
  of anything that changed. Failure mode: a **missed** duplicate folder, with
  nothing appearing to be wrong.

**Resolution — tier the folder pass too.** Give folders a stat-only signature and
use it as the cheap bucketing key, exactly as `size` is for files:

```
folderSig(F) = H( sorted( ["f:" + size   for each file child]
                        + [folderSig(sub) for each subfolder child] ) )
```

Zero bytes read, still content-only and name-agnostic, and — critically —
**intrinsic**: a pure function of the subtree, therefore stable across scans and
scan scopes, which makes §4.3's ancestor-chain invalidation correct again.

Then apply one rule that removes the degraded token entirely:

> **A folder content hash is only ever computed from fully-hashed children.**

Only signature-colliding folders need a real content hash, and computing one forces
hashing of that folder's file children — a bounded, targeted set rather than the
whole tree. Tokens are therefore always `size:realhash`, never degraded, and
`folder_hash` is intrinsic again. `folders.folder_hash` becomes `NULL` for folders
whose signature is unique (provably not duplicates).

**Caveat:** folder-signature collisions will be far more common than file-size
collisions (every folder holding a single zero-byte file collides with every
other). Resolving them means hashing contents, which a genuine match requires
anyway, and the candidate set stays small relative to the tree.

**Structural note:** file-level and folder-level dedup now have *different* tier-0
keys (`size` vs `folderSig`). They are independent passes over the same `stat`
data; neither prunes the other, since a file inside a signature-unique folder can
still duplicate a file elsewhere.

#### 12.8.1 Existing soundness gap this would close

Independent of everything above: **folder duplicates are currently never
verified.** `dupes.Report` and `dupes.CandidateReport` both take `FolderGroups`
straight from `store.FolderDupGroups`, and `v1.FolderGroupDTO` has no `Verified`
field or verification path. Since `folder_hash` is built from **head** hashes, two
folders whose files merely share sizes and identical first 64 KiB are reported as
duplicate folders with no way to confirm them — and §7's rail (*no destructive
action ever runs on an unverified group*) has **no folder implementation at all**.
This becomes materially dangerous in Phase 4, when trashing a whole folder is
possible.

Building tokens from tier-2 **full** hashes makes folder equality exactly as strong
as file equality and closes this gap. This is a second, independent argument for
adopting §12.2.

### 12.9 Suggested order

1. **§12.5** — fix the representative re-read. Pure win, no design change.
2. **§12.8.1** — close the unverified-folder-duplicate gap, or at minimum surface
   folder groups as unconfirmed in the UI. Safety, and a Phase 4 blocker.
3. **§12.8** — adopt tiered folder signatures. This is the precondition that makes
   Decision B safe.
4. **§12.3** — settle whether every file gets a head hash.
5. **§12.2** — adopt or reject tiered hashing (file tiers + full-hash tokens).
6. **§12.4** — confirm the pre-action byte-compare stays (expected: yes).
7. **§12.7** — cross-scan history / diff / cross-root dedup, independently.

Do this while the phase is still read-only; steps 3–5 change what `verified` means
in the `v1` DTOs, and steps 2 and 6 are preconditions for Phase 4.
