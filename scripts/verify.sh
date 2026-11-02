#!/usr/bin/env bash
# GeoPilot verification helper (macOS / Linux / Git Bash)
#
# Usage:
#   ./scripts/verify.sh
#   TEST_DB_DSN='...' ./scripts/verify.sh
#
# Integration tests require TEST_DB_DSN pointing at a disposable MySQL
# database. They clear application tables in that database.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

echo '==> gofmt'
UNFORMATTED="$(find core backend -name '*.go' -type f -print0 | xargs -0 gofmt -l)"
if [[ -n "$UNFORMATTED" ]]; then
  printf 'Go files need formatting:\n%s\n' "$UNFORMATTED" >&2
  exit 1
fi

echo '==> go vet'
go vet ./core/... ./backend/...

if [[ -n "${TEST_DB_DSN:-}" ]]; then
  echo '    TEST_DB_DSN is set; MySQL integration tests will run.'
else
  echo '    TEST_DB_DSN is not set; MySQL integration tests will be skipped.'
fi

echo '==> go test'
go test -count=1 ./core/... ./backend/...

echo '==> frontend build'
(cd frontend && npm run build)

echo
echo 'Verification passed.'
