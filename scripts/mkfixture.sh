#!/usr/bin/env bash
#
# mkfixture.sh — build a synthetic tree for exercising the scan/classify pipeline.
#
# The tree deliberately contains every case the design calls out: exact file
# duplicates, content-identical folders with differing names, empty folders,
# zero-byte files, a head-hash collision (same size + same first 64 KiB, different
# tail), hidden entries, and an unreadable file.
#
# Usage: scripts/mkfixture.sh [dest] [--force]
#
# Refuses to touch an existing directory unless it was created by this script
# (identified by the .hashdupes-fixture marker) and --force is given.

set -euo pipefail

DEST="${1:-/tmp/hashdupes-fixture}"
FORCE=0
for arg in "$@"; do
  [[ "$arg" == "--force" ]] && FORCE=1
done

MARKER=".hashdupes-fixture"
HEAD_BYTES=65536 # must match hash.DefaultHeadBytes

if [[ -e "$DEST" ]]; then
  if [[ "$FORCE" != 1 ]]; then
    echo "error: $DEST already exists (pass --force to rebuild)" >&2
    exit 1
  fi
  if [[ ! -f "$DEST/$MARKER" ]]; then
    echo "error: $DEST exists but has no $MARKER; refusing to remove it" >&2
    exit 1
  fi
  chmod -R u+rwX "$DEST"
  rm -rf "$DEST"
fi

# fill <char> <bytes> — deterministic filler so runs are reproducible.
fill() { head -c "$2" /dev/zero | tr '\0' "$1"; }

mkdir -p "$DEST"
cd "$DEST"
touch "$MARKER"

# --- exact file duplicates, same folder and across folders -------------------
mkdir -p docs nested/deep
fill a 4096 >docs/unique-a.bin
fill b 8192 >docs/report.bin
fill b 8192 >nested/deep/report-copy.bin # dup of docs/report.bin, different name

# --- a three-copy group -----------------------------------------------------
mkdir -p triple
for n in 1 2 3; do fill c 2048 >"triple/copy-$n.bin"; done

# --- content-identical folders with different folder and file names ---------
# Folder hashing is content-only, so these two must land in one group.
mkdir -p album-2019 backup/album-copy
fill d 3000 >album-2019/one.bin
fill e 5000 >album-2019/two.bin
fill d 3000 >backup/album-copy/first.bin
fill e 5000 >backup/album-copy/second.bin

# --- empty folders (canonical empty hash => they group together) ------------
mkdir -p empty-one empty-two nested/also-empty

# --- zero-byte files (share size 0 + empty head hash) ----------------------
mkdir -p zero
: >zero/placeholder
: >zero/.keep-alt
: >docs/empty-note.txt

# --- head-hash collision: identical first 64 KiB, divergent tail -----------
# These bucket together on (size, head_hash) but must be rejected by the
# byte-compare. This is the case the verification step exists for.
mkdir -p collision
{ fill x "$HEAD_BYTES"; fill y 32768; } >collision/same-head-1.bin
{ fill x "$HEAD_BYTES"; fill z 32768; } >collision/same-head-2.bin

# --- a genuine duplicate larger than the head window -----------------------
# Confirms verification passes on files whose tails also match.
mkdir -p media
{ fill m "$HEAD_BYTES"; fill n 1048576; } >media/clip.bin
{ fill m "$HEAD_BYTES"; fill n 1048576; } >media/clip-copy.bin

# --- hidden entries (exercise the ignoreHidden option) ---------------------
mkdir -p .cache
fill h 1024 >.cache/hidden-dup.bin
fill h 1024 >docs/visible-twin.bin # dup only when hidden files are included

# --- unreadable file (exercise skip accounting) ---------------------------
mkdir -p locked
fill q 1024 >locked/no-read.bin
chmod 000 locked/no-read.bin

cat <<EOF
fixture built at: $DEST

expected duplicate folders:
  album-2019 == backup/album-copy   (content-only match, names differ)
  empty-one == empty-two == nested/also-empty

expected duplicate file sets:
  docs/report.bin == nested/deep/report-copy.bin
  triple/copy-{1,2,3}.bin           (3 copies)
  media/clip.bin == media/clip-copy.bin
  zero/placeholder == docs/empty-note.txt          (zero-byte; joined by
                                                    zero/.keep-alt only when
                                                    hidden files are included)
  the album-2019 / backup pairs     (should be flagged withinDupFolder)
  .cache/hidden-dup.bin == docs/visible-twin.bin  (only with hidden included)

expected to be REJECTED by byte-compare:
  collision/same-head-{1,2}.bin     (same size + first ${HEAD_BYTES} bytes)

expected to be skipped:
  locked/no-read.bin                (unreadable; not skipped when run as root)
EOF
