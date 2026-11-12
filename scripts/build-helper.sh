#!/bin/sh
set -eu
case "${ROCKETSCIENCE_ARCH:-$(uname -m)}" in
  arm64) swift_arch=arm64; go_arch=arm64 ;;
  x86_64|amd64) swift_arch=x86_64; go_arch=amd64 ;;
  *) echo "不支持的 macOS 处理器架构" >&2; exit 1 ;;
esac
export MACOSX_DEPLOYMENT_TARGET=13.0
swift_target="${swift_arch}-apple-macosx${MACOSX_DEPLOYMENT_TARGET}"
out=resources/darwin/bin
mkdir -p "$out"
xcrun swiftc -target "$swift_target" native/SystemProxy/main.swift -o "$out/rs-system-proxy"
GOOS=darwin GOARCH="$go_arch" go build -trimpath -o "$out/rs-tun-helper" ./cmd/tun-helper
xcrun swiftc -target "$swift_target" -emit-library -module-name RocketNetwork -Xlinker -install_name -Xlinker '@rpath/libRocketNetwork.dylib' native/Network/Network.swift -o "$out/libRocketNetwork.dylib"
