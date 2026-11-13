# Building Rocket Science from Source

## Prerequisites

- **Go**: `1.27.3` (see `go.mod`)
- **Bun**: package manager and script runner
- **macOS packaging**: Xcode Command Line Tools (`clang`, `codesign`, `hdiutil`)
- **Windows installer**: `makensis` (NSIS) for installer builds
- **Linux runtime / tests**: `libwebkit2gtk-4.1-0` and `libgtk-3-0` (Docker is optional for running Linux tests on other hosts)

## Development & Frontend Commands

Install JavaScript dependencies:

```sh
bun install
```

Run in development mode (builds the macOS helper and starts `mygo dev` with `GOFLAGS=-tags=with_gvisor`):

```sh
bun run dev
```

Frontend verification and build:

```sh
bun run typecheck
bun run verify:components
bun run build:web
```

## Release Packaging

Build the macOS app bundle:

```sh
bun run build
```

Build release artifacts for one or more platforms (`darwin/arm64`, `darwin/amd64`, `windows/amd64`, `linux/amd64`):

```sh
bun run build:release --platform=darwin/arm64
```

Outputs are written under `build.noindex/<platform>-<arch>/` (for example `build.noindex/darwin-arm64/`).

## Go Tests

Run the Go test suite with gVisor TUN support enabled:

```sh
GOFLAGS=-tags=with_gvisor go test ./...
```

To run the real-mihomo integration checks in `internal/core`, set `ROCKETSCIENCE_MIHOMO` to the path of a `mihomo` binary:

```sh
ROCKETSCIENCE_MIHOMO=/path/to/mihomo GOFLAGS=-tags=with_gvisor go test ./internal/core/...
```
