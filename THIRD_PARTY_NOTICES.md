# Third-party notices

**Rocket Science** (source in this repository) is licensed under the **GNU General Public License v3.0 or later**. The full license text is in [LICENSE](LICENSE).

When you distribute binaries built from this project, you must preserve license and attribution materials in the application bundle (see `bun run verify:bundle`) and comply with the licenses of bundled and linked components below.

## Bundled inside release `.app` artifacts

| Component | Version (pinned in build) | License | Where the notice lives |
|-----------|---------------------------|---------|-------------------------|
| mihomo (MetaCubeX) | v1.19.33 | GPL-3.0 | `resources/licenses/mihomo-GPL-3.0.txt`; copied into `RocketScience.app/Contents/Resources/licenses/` |

## Vendored source in this repository

| Component | Location | License |
|-----------|----------|---------|
| sing-tun (fork/replace) | `third_party/sing-tun` | GPL-3.0 — see `third_party/sing-tun/LICENSE` |

Modifications to vendored sing-tun are tracked in Git history; upstream is [metacubex/sing-tun](https://github.com/metacubex/sing-tun).

## Build-time and linked dependencies (representative)

| Component | Role | License (upstream) |
|-----------|------|-------------------|
| MyGo (`github.com/egoist/mygo`) | macOS desktop shell, WebView, menu bar | See [MyGo repository](https://github.com/egoist/mygo) |
| React / React DOM | UI | MIT |
| mihomo module (`github.com/metacubex/mihomo`) | In-process core (build tag) | GPL-3.0 |
| Other Go modules | Networking, YAML, etc. | See `go.mod` and each module’s upstream |

This list is not exhaustive. For source distribution, include this file, `LICENSE`, and the license files already shipped under `resources/licenses/` and `third_party/`.

## Privileged helper

The `rs-tun-helper` helper source lives under `cmd/tun-helper/` and is built into the app bundle; it is part of Rocket Science under GPL-3.0 alongside the main application.
