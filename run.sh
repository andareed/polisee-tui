#!/usr/bin/env bash
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")"
go build -buildvcs=false -o bin/polisee-tui .
if [[ "${1:-}" == "build" ]]; then
  exit 0
fi
exec ./bin/polisee-tui "$@"
