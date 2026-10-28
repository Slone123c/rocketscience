#!/bin/sh
set -eu
: "${ROCKETSCIENCE_MIHOMO:?Set ROCKETSCIENCE_MIHOMO to the mihomo executable}"
go test ./...
