#!/usr/bin/env bash
. "$(dirname "$0")/_common.sh"
# 默认 10 并发 30 秒，覆盖时只需追加参数，如：bench.sh -c 50 -d 1m
if [ "$#" -eq 0 ]; then
  set -- -c 10 -d 30s
fi
exec "$BIN" bench --base "$TB_BASE" --token "$TB_TOKEN" "$@"
