# Third-party notices

**Rocket Science** first-party source in this repository is licensed under the **GNU General Public License version 3 only**. The full license text is in [LICENSE](LICENSE).

When you distribute binaries built from this project, you must preserve license and attribution materials in the application bundle. Before each macOS build, `scripts/collect-licenses.ts` copies pinned npm and Go module licenses into `resources/licenses/` (see `MANIFEST.json`); `bun run verify:bundle` checks they are present in `RocketScience.app/Contents/Resources/licenses/`.

## Bundled inside release `.app` artifacts

| Component | Version (pinned in build) | License | Where the notice lives |
|-----------|---------------------------|---------|-------------------------|
| mihomo (MetaCubeX) | v1.19.33 | GPL-3.0 | `resources/licenses/mihomo-GPL-3.0.txt`; copied into `RocketScience.app/Contents/Resources/licenses/` |

### Bundled geo / rule databases (upstream; not Rocket Science–authored)

Release builds copy Mihomo-compatible rule databases into `resources/` so the app can start without downloading them. **These binaries are upstream data**, not original work in this repository.

| File in app bundle | Upstream | Default fetch URL in core | Upstream license | Pinned copy (`resources/`) |
|--------------------|----------|---------------------------|------------------|----------------------------|
| `geosite.dat` | [MetaCubeX/meta-rules-dat](https://github.com/MetaCubeX/meta-rules-dat) | `https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geosite.dat` (`defaultGeoX` in `internal/core/core_options.go`) | **GPL-3.0-only** — [LICENSE on `master`](https://github.com/MetaCubeX/meta-rules-dat/blob/master/LICENSE) | SHA-256 `c2ec1559aa5de2420f61ced348dd45ef44f07673d3bfc95dfab86174db3fd926` (4 354 536 bytes) |
| `geoip.metadb` | same | `https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geoip.metadb` | same | SHA-256 `106c69bb0d8cf413a970bf958f14b0c80271e581af4c5bdd4958b9a98f4d5211` (8 597 075 bytes) |

Refresh these files from the upstream `release` branch (or the jsDelivr URLs above) when updating bundles, then update the hashes in this table. At documentation time, live `@release` on jsDelivr may already differ from the pinned copies shipped in Git.

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
