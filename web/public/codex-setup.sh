#!/usr/bin/env bash
set -euo pipefail

umask 077

api_key=''
base_url='https://gate.vectorepoch.com/v1'
model='gpt-5.5'

usage() {
  cat <<'EOF'
向量纪元 Relay Codex 配置脚本

用法:
  codex-setup.sh --api-key <key> [--base-url <url>] [--model <model>]
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --api-key)
      [ "$#" -ge 2 ] || { echo '缺少 --api-key 的值' >&2; exit 2; }
      api_key="$2"
      shift 2
      ;;
    --base-url)
      [ "$#" -ge 2 ] || { echo '缺少 --base-url 的值' >&2; exit 2; }
      base_url="$2"
      shift 2
      ;;
    --model)
      [ "$#" -ge 2 ] || { echo '缺少 --model 的值' >&2; exit 2; }
      model="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "未知参数: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [ -z "$api_key" ]; then
  printf '请输入 Relay API Key: '
  read -r -s api_key
  printf '\n'
fi

case "$api_key" in
  sk-*) ;;
  *) echo 'API Key 格式不正确，应以 sk- 开头' >&2; exit 2 ;;
esac

case "$base_url" in
  https://*) ;;
  *) echo 'API 地址必须使用 HTTPS' >&2; exit 2 ;;
esac

command -v curl >/dev/null 2>&1 || {
  echo '未找到 curl，请先安装 curl 后重试。' >&2
  exit 1
}

mask_api_key() {
  if [ "${#api_key}" -le 12 ]; then
    printf '%s...%s' "${api_key:0:4}" "${api_key: -4}"
    return
  fi
  printf '%s...%s' "${api_key:0:8}" "${api_key: -6}"
}

echo '正在验证 Relay API Key...'
models_url="${base_url%/}/models"
if ! printf 'header = "Authorization: Bearer %s"\n' "$api_key" | \
  curl -fsS --connect-timeout 10 --max-time 30 --config - "$models_url" >/dev/null 2>&1; then
  echo 'Relay API Key 验证失败，请检查 Key 是否有效或网络是否可用。' >&2
  exit 1
fi

if ! command -v codex >/dev/null 2>&1; then
  echo '未找到 Codex CLI，正在使用官方安装脚本安装。'
  curl -fsSL https://chatgpt.com/codex/install.sh | CODEX_NON_INTERACTIVE=1 sh
  export PATH="$HOME/.local/bin:$PATH"
fi

command -v codex >/dev/null 2>&1 || {
  echo 'Codex CLI 安装后仍不可用，请重新打开终端后重试。' >&2
  exit 1
}

codex_home="${CODEX_HOME:-$HOME/.codex}"
mkdir -p "$codex_home"
chmod 700 "$codex_home"

timestamp="$(date +%Y%m%d-%H%M%S)"
backup_dir="$codex_home/backups"
mkdir -p "$backup_dir"

if [ -e "$codex_home/config.toml" ]; then
  cp -p "$codex_home/config.toml" "$backup_dir/config.toml.$timestamp.bak"
fi

if [ -e "$codex_home/auth.json" ]; then
  cp -p "$codex_home/auth.json" "$backup_dir/auth.json.$timestamp.bak"
fi

temp_dir="$(mktemp -d "${TMPDIR:-/tmp}/vectorepoch-codex.XXXXXX")"
trap 'rm -rf "$temp_dir"' EXIT

cat > "$temp_dir/config.toml" <<EOF
# 由向量纪元 Relay 生成。重新执行配置命令即可更新。
model_provider = "vectorepoch"
model = "$model"
review_model = "$model"
model_reasoning_effort = "xhigh"
sandbox_mode = "workspace-write"

[model_providers.vectorepoch]
name = "向量纪元 Relay"
base_url = "$base_url"
wire_api = "responses"
requires_openai_auth = true

[sandbox_workspace_write]
network_access = true

[features]
goals = true
EOF

mv "$temp_dir/config.toml" "$codex_home/config.toml"
chmod 600 "$codex_home/config.toml"

printf '%s\n' "$api_key" | codex login --with-api-key

if ! grep -Fqx 'model_provider = "vectorepoch"' "$codex_home/config.toml" || \
  ! grep -Fqx "base_url = \"$base_url\"" "$codex_home/config.toml" || \
  ! codex login status >/dev/null 2>&1; then
  echo 'Codex 配置校验失败，未确认配置已生效。' >&2
  exit 1
fi

echo 'Codex 已配置完成。'
printf '已配置 Key: %s\n' "$(mask_api_key)"
echo "配置文件: $codex_home/config.toml"
if [ -e "$backup_dir/config.toml.$timestamp.bak" ] || [ -e "$backup_dir/auth.json.$timestamp.bak" ]; then
  echo "旧配置备份: $backup_dir/*.$timestamp.bak"
fi
