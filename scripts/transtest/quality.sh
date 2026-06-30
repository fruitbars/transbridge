#!/usr/bin/env bash
. "$(dirname "$0")/_common.sh"
GOLDEN="${TB_GOLDEN:-$ROOT/cmd/transtest/testdata/golden.yml}"
exec "$BIN" quality --base "$TB_BASE" --token "$TB_TOKEN" --golden "$GOLDEN" "$@"
