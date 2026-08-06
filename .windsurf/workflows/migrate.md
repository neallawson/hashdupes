---
description: Manage the hashdupes index database schema migrations
---

Migrations are embedded in `internal/index/migrations` and run automatically when
the app starts. Use these commands for explicit/standalone control via the
`cmd/migrate` tool (goose-backed).

Show current migration status:
// turbo
```
make migrate-status
```

Apply all pending migrations:
// turbo
```
make migrate-up
```

Roll back the most recent migration:
```
make migrate-down
```

Target a specific database file instead of the default per-user path:
```
go run ./cmd/migrate -db /path/to/hashdupes.db status
```

To add a new migration, create a file in `internal/index/migrations/` named
`NNNN_description.sql` (incrementing the numeric prefix) using goose's
`-- +goose Up` / `-- +goose Down` annotations, then run `make migrate-up`.
