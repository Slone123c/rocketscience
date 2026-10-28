#!/bin/sh
set -eu
mkdir -p resources/bin
xcrun swiftc native/SystemProxy/main.swift -o resources/bin/rs-system-proxy
go build -o resources/bin/rs-tun-supervisor ./cmd/tun-supervisor
