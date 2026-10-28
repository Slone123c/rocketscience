#!/bin/sh
set -eu
version=v1.19.33
case "$(uname -m)" in
  arm64) arch=arm64; expected=060b64b0f2c9a310831db0fe4adb2878de67bfa606c311813c1f13966aa504b6 ;;
  x86_64) arch=amd64; expected=551ae54ef8d77a5e21f7a8185a59b441fb13a0fb0514fcdef5ca2816f6f50015 ;;
  *) echo "不支持的 macOS 处理器架构" >&2; exit 1 ;;
esac
mkdir -p .cache/mihomo resources/bin
asset="mihomo-darwin-${arch}-${version}.gz"
archive=".cache/mihomo/$asset"
if [ ! -f "$archive" ] || [ "$(shasum -a 256 "$archive" | cut -d " " -f 1)" != "$expected" ]; then
  temp=$(mktemp "${TMPDIR:-/tmp}/rocket-core.XXXXXX")
  trap 'rm -f "$temp"' EXIT HUP INT TERM
  curl --fail --location --silent --show-error "https://github.com/MetaCubeX/mihomo/releases/download/${version}/${asset}" -o "$temp"
  [ "$(shasum -a 256 "$temp" | cut -d " " -f 1)" = "$expected" ] || { echo "mihomo 文件校验失败" >&2; exit 1; }
  mv "$temp" "$archive"
fi
out=$(mktemp "${TMPDIR:-/tmp}/rocket-bin.XXXXXX")
trap 'rm -f "$out"' EXIT HUP INT TERM
gzip -dc "$archive" > "$out"
chmod 755 "$out"
mv "$out" resources/bin/mihomo
