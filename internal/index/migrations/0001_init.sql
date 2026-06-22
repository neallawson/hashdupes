-- +goose Up
-- +goose StatementBegin

-- A single recursive indexing session over a root directory.
CREATE TABLE scans (
    id          INTEGER PRIMARY KEY,
    root_path   TEXT    NOT NULL,
    algo        TEXT    NOT NULL,            -- 'blake3' | 'sha256'
    head_bytes  INTEGER NOT NULL,            -- leading bytes hashed per file
    status      TEXT    NOT NULL,            -- 'running' | 'done' | 'error' | 'canceled'
    started_at  INTEGER NOT NULL,            -- unix nanoseconds
    finished_at INTEGER                      -- unix nanoseconds, null while running
);

-- Indexed directories. folder_hash is a content-only Merkle hash.
CREATE TABLE folders (
    id          INTEGER PRIMARY KEY,
    scan_id     INTEGER NOT NULL REFERENCES scans(id) ON DELETE CASCADE,
    parent_id   INTEGER REFERENCES folders(id) ON DELETE CASCADE,
    path        TEXT    NOT NULL,
    name        TEXT    NOT NULL,
    folder_hash BLOB,
    is_empty    INTEGER NOT NULL DEFAULT 0,  -- 1 if no non-skipped children
    file_count  INTEGER NOT NULL DEFAULT 0,
    total_size  INTEGER NOT NULL DEFAULT 0
);

-- Indexed regular files. head_hash is always populated once hashed.
CREATE TABLE files (
    id           INTEGER PRIMARY KEY,
    scan_id      INTEGER NOT NULL REFERENCES scans(id) ON DELETE CASCADE,
    parent_id    INTEGER REFERENCES folders(id) ON DELETE CASCADE,
    path         TEXT    NOT NULL,
    name         TEXT    NOT NULL,
    size         INTEGER NOT NULL,
    mtime        INTEGER NOT NULL,           -- unix nanoseconds
    ctime        INTEGER,                    -- best-effort, platform-dependent
    ext          TEXT,
    is_empty     INTEGER NOT NULL DEFAULT 0, -- 1 if size == 0
    head_hash    BLOB,
    verified_grp INTEGER,                    -- nullable: confirmed dup-group id
    hashed_at    INTEGER
);

-- Persistent, cross-scan cache so unchanged files are never re-hashed.
CREATE TABLE hash_cache (
    path       TEXT    NOT NULL,
    size       INTEGER NOT NULL,
    mtime      INTEGER NOT NULL,
    algo       TEXT    NOT NULL,
    head_bytes INTEGER NOT NULL,
    head_hash  BLOB    NOT NULL,
    PRIMARY KEY (path, size, mtime, algo, head_bytes)
);

CREATE INDEX idx_files_scan    ON files(scan_id);
CREATE INDEX idx_files_parent  ON files(parent_id);
CREATE INDEX idx_files_dup     ON files(scan_id, size, head_hash); -- candidate grouping
CREATE INDEX idx_folders_scan  ON folders(scan_id);
CREATE INDEX idx_folders_parent ON folders(parent_id);
CREATE INDEX idx_folders_hash  ON folders(scan_id, folder_hash);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS hash_cache;
DROP TABLE IF EXISTS files;
DROP TABLE IF EXISTS folders;
DROP TABLE IF EXISTS scans;
-- +goose StatementEnd
