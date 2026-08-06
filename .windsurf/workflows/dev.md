---
description: Run hashdupes in development mode with hot reload
---

Prerequisites (one-time):

1. Install the Wails CLI (pinned to the module's wails/v2 version):
// turbo
```
make tools
```

2. Ensure native dependencies are present (Linux requires GTK + WebKit). Check with:
// turbo
```
make doctor
```
If any required package is "Available" (not "Installed"), install it, e.g. on Ubuntu:
```
sudo apt install libgtk-3-dev libwebkit2gtk-4.0-dev
```

3. Install frontend dependencies:
// turbo
```
make deps
```

Run the app with hot reload. This starts the Vite dev server and the Go backend,
rebuilding on changes:
```
make dev
```

Notes:
- The index database lives in the per-user config dir (e.g. ~/.config/hashdupes/hashdupes.db)
  and migrations run automatically on startup.
- This phase is read-only: scanning and duplicate viewing only; no files are moved or deleted.
