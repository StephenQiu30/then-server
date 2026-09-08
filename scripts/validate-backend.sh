#!/bin/sh
set -eu
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(dirname -- "$script_dir")
mode=${1:-unit}
case "$mode" in unit|integration) ;; *) printf '%s\n' 'usage: validate-backend.sh [unit|integration]' >&2; exit 2 ;; esac
[ "$#" -le 1 ] || exit 2
cd "$repo_root/backend"
[ "$(go env GOVERSION)" = 'go1.26.5' ] || { printf '%s\n' 'Go 1.26.5 is required' >&2; exit 1; }
unformatted=$(gofmt -l .)
[ -z "$unformatted" ] || { printf '%s\n' "$unformatted" >&2; exit 1; }
go mod verify
go vet ./...
go test ./...
go test -race ./...
if [ "$mode" = integration ]; then
    go test -race -tags=integration ./tests -count=1
fi
printf '%s\n' 'Backend checks passed; this is not production or Swift Client acceptance.'
