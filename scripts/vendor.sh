#!/usr/bin/env bash
# Re-vendor the server's Go dependencies from the published module versions (ignores any local
# go.work) and restore the C header directories that `go mod vendor` drops.
#
# `go mod vendor` copies only directories that contain Go files, so a cgo dependency whose sources
# #include a header-only directory (smacker/go-tree-sitter: php/tree_sitter, sql/tree_sitter) no
# longer compiles from vendor/. This script copies such directories back from the module cache.
set -euo pipefail

cd "$(dirname "$0")/../server"
export GOWORK=off

go mod tidy
go mod vendor

restored=0
while read -r module; do
  src="$(go list -mod=mod -m -f '{{.Dir}}' "$module" 2>/dev/null || true)"
  dst="vendor/$module"
  [ -n "$src" ] && [ -d "$src" ] && [ -d "$dst" ] || continue
  # Only cgo modules (they ship .c files) can need headers that were dropped.
  [ -n "$(find "$src" -name '*.c' -print -quit)" ] || continue

  while read -r dir; do
    rel="${dir#"$src"/}"
    [ -e "$dst/$rel" ] && continue
    # header-only directory: has headers, no Go files
    [ -n "$(find "$dir" -maxdepth 1 -name '*.h' -print -quit)" ] || continue
    [ -z "$(find "$dir" -maxdepth 1 -name '*.go' -print -quit)" ] || continue
    mkdir -p "$dst/$rel"
    cp -R "$dir"/. "$dst/$rel"/
    chmod -R u+w "$dst/$rel"
    echo "restored $module/$rel"
    restored=$((restored + 1))
  done < <(find "$src" -type d)
done < <(awk '/^# /{print $2}' vendor/modules.txt)

echo "vendor ready ($restored header directories restored)"
