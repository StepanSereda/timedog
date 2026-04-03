# Contributing

Thanks for helping improve timedog and timedog-server.

## Before you start

- Read [DISCLAIMER.md](DISCLAIMER.md): end users and contributors accept that AI-assisted parts are experimental and that **risk stays with the user**.
- Read [.github/COPYRIGHT.md](COPYRIGHT.md) for license boundaries (GPL-2.0+ for most of the repo; `timecopy.py` is GPL-3.0).
- The Go server aims to match the **behaviour** of the Perl `timedog` (inode-aware comparison of two snapshot trees), not necessarily identical terminal formatting.

## Development

- **Go:** `go test ./...`, `go vet ./...` from the repository root.
- **Web UI:** `cd web && npm install && npm run build` — output is embedded under `cmd/timedog-server/web/dist` for release builds.

## Pull requests

- Describe what changed and why (Time Machine / snapshot edge cases welcome).
- Keep unrelated drive-by refactors out of the same PR when possible.

## Issues

Use the bug report template when something breaks; include macOS version, how you run the server, and whether Full Disk Access is relevant.
