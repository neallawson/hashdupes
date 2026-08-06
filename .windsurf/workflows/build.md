---
description: Produce a production build of the hashdupes desktop app
---

Prerequisites: Wails CLI installed (`make tools`) and native deps present (`make doctor`).

1. Run the full test suite first:
// turbo
```
make test
```

2. Type-check the frontend:
// turbo
```
make check-frontend
```

3. Produce a clean production build. The binary is written to `build/bin/hashdupes`:
```
make build
```

For a debug build with devtools enabled instead:
```
make build-debug
```

The frontend is compiled by Vite into `frontend/dist` and embedded into the Go binary,
so the resulting executable is self-contained.
