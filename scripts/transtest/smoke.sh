#!/usr/bin/env bash
. "$(dirname "$0")/_common.sh"
exec "$BIN" smoke --base "$TB_BASE" --token "$TB_TOKEN" "$@"
