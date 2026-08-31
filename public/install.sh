#!/usr/bin/env bash
set -euo pipefail

api_origin="${VSEARCH_API_ORIGIN:-https://gate.vectorepoch.com}"
api_key="${VSEARCH_API_KEY:-}"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --key) api_key="${2:-}"; shift 2 ;;
    --origin) api_origin="${2:-}"; shift 2 ;;
    *) printf '未知参数：%s\n' "$1" >&2; exit 1 ;;
  esac
done
if ! command -v node >/dev/null 2>&1; then
  printf '%s\n' '需要 Node.js 18 或更高版本才能写入 MCP 配置。' >&2
  exit 1
fi

installer_file="$(mktemp "${TMPDIR:-/tmp}/vsearch-install.XXXXXX.mjs")"
trap 'rm -f "$installer_file"' EXIT
curl -fsSL "${api_origin%/}/install.mjs" -o "$installer_file"
VSEARCH_API_KEY="$api_key" VSEARCH_API_ORIGIN="$api_origin" node "$installer_file"
