#!/usr/bin/env bash

# 生成开放接口 API Key 及其配置摘要。明文 Key 仅会输出一次，不应写入仓库或日志。
# 用法：
#   OPENAPI_PEPPER='<OpenApi.Pepper>' ./script/gen_openapi_key.sh -u ds-tenant-a
#   OPENAPI_PEPPER='<OpenApi.Pepper>' ./script/gen_openapi_key.sh -n 2 -s psych:report -u ds-tenant-a

set -euo pipefail

readonly secret_length=36
count=1
scopes='psych:chat,psych:report'
upstream=''

usage() {
  cat <<'EOF'
用法：OPENAPI_PEPPER='<OpenApi.Pepper>' ./script/gen_openapi_key.sh -u 上游名 [-n 数量] [-s 权限列表]

  -u  上游 Key 名，须与 OpenApi.Upstreams 中的条目一致；必填
  -n  生成数量，默认 1
  -s  逗号分隔的权限列表，默认 psych:chat,psych:report
EOF
}

while getopts ':n:s:u:h' option; do
  case "$option" in
    n) count="$OPTARG" ;;
    s) scopes="$OPTARG" ;;
    u) upstream="$OPTARG" ;;
    h)
      usage
      exit 0
      ;;
    *)
      usage >&2
      exit 1
      ;;
  esac
done

if [[ -z "${OPENAPI_PEPPER:-}" ]]; then
  echo '错误：请通过 OPENAPI_PEPPER 提供与 OpenApi.Pepper 一致的服务器盐。' >&2
  exit 1
fi
if ! [[ "$count" =~ ^[1-9][0-9]*$ ]]; then
  echo '错误：-n 必须是正整数。' >&2
  exit 1
fi
if [[ -z "$scopes" ]]; then
  echo '错误：-s 不能为空。' >&2
  exit 1
fi
if [[ -z "$upstream" ]]; then
  echo '错误：必须通过 -u 指定上游 Key 名，且须与 OpenApi.Upstreams 中的条目一致。' >&2
  exit 1
fi

yaml_scopes="[$(printf '%s' "$scopes" | sed 's/,/, /g')]"

for ((i = 1; i <= count; i++)); do
  secret=$(LC_ALL=C openssl rand -base64 64 | tr -dc 'A-Za-z0-9' | cut -c "1-${secret_length}")
  if [[ ${#secret} -ne $secret_length ]]; then
    echo '错误：随机密钥生成失败，请重试。' >&2
    exit 1
  fi
  prefix=$(printf '%s' "$secret" | openssl dgst -sha256 | awk '{print $NF}' | cut -c 1-8)
  digest=$(printf '%s' "$secret" | openssl dgst -sha256 -hmac "$OPENAPI_PEPPER" | awk '{print $NF}')

  printf '# ===== 第 %d 把 Key（仅展示一次，请勿写入日志或仓库）=====\n' "$i"
  printf 'API Key: sk_live_%s_%s\n' "$prefix" "$secret"
  printf '配置片段：\n'
  printf '  - Prefix: %s\n    Digest: %s\n    Status: active\n    Scopes: %s\n    Upstream: %s\n\n' \
    "$prefix" "$digest" "$yaml_scopes" "$upstream"
done
