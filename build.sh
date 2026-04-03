#!/usr/bin/env bash
# Единая сборка: Vite → cmd/timedog-server/web/dist → go:embed → бинарник с раздачей статики.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WEB="$ROOT/web"
EMBED_DIST="$ROOT/cmd/timedog-server/web/dist"
OUT="${1:-$ROOT/timedog-server}"

cd "$ROOT"

if [[ ! -d "$WEB" ]]; then
  echo "error: missing $WEB" >&2
  exit 1
fi

echo "==> Frontend: npm (web/)"
if [[ -f "$WEB/package-lock.json" ]]; then
  (cd "$WEB" && npm ci)
else
  (cd "$WEB" && npm install)
fi

echo "==> Frontend: production build"
(cd "$WEB" && npm run build)

if [[ ! -d "$WEB/dist" ]] || [[ ! -f "$WEB/dist/index.html" ]]; then
  echo "error: $WEB/dist is missing or empty after vite build" >&2
  exit 1
fi

echo "==> Copy static → $EMBED_DIST (for //go:embed)"
rm -rf "$EMBED_DIST"
mkdir -p "$EMBED_DIST"
cp -R "$WEB/dist/"* "$EMBED_DIST/"

echo "==> Go build → $OUT"
go build -o "$OUT" ./cmd/timedog-server

echo "Done. Run: $OUT"
