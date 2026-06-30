#!/usr/bin/env bash
# 共享初始化：定位仓库根，定位二进制，加载 .env
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BIN="$ROOT/dist/transtest"

if [ ! -x "$BIN" ]; then
  echo "错误: 找不到 $BIN，请先在仓库根运行 ./build.sh" >&2
  exit 1
fi

ENV_FILE="$ROOT/scripts/transtest/.env"
if [ -f "$ENV_FILE" ]; then
  set -a
  # shellcheck disable=SC1090
  . "$ENV_FILE"
  set +a
fi

: "${TB_BASE:=http://localhost:8080}"
: "${TB_TOKEN:=}"
