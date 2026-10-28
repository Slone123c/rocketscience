#!/bin/sh
set -eu
case "$(uname -m)" in
  arm64) swift_arch=arm64 ;;
  x86_64) swift_arch=x86_64 ;;
  *) echo "不支持的 macOS 处理器架构" >&2; exit 1 ;;
esac
# Match the app's declared minimum and the Go toolchain's macOS requirement.
export MACOSX_DEPLOYMENT_TARGET=13.0
swift_target="${swift_arch}-apple-macosx${MACOSX_DEPLOYMENT_TARGET}"
mkdir -p resources/bin
xcrun swiftc -target "$swift_target" native/SystemProxy/main.swift -o resources/bin/rs-system-proxy
go build -o resources/bin/rs-tun-supervisor ./cmd/tun-supervisor
xcrun swiftc -target "$swift_target" -emit-library -module-name RocketNetwork -Xlinker -install_name -Xlinker '@rpath/libRocketNetwork.dylib' native/Network/Network.swift -o resources/bin/libRocketNetwork.dylib
