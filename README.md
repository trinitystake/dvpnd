# dvpnd

[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/trinitystake/dvpnd)](go.mod)

`dvpnd` is an open source dVPN node daemon for the Sentinel blockchain (chain ID
`sentinelhub-2`). It registers the node on-chain, serves WireGuard or V2Ray sessions to
subscribers, and reports usage. It is licensed under the Apache License 2.0 and is not
affiliated with Sentinel or Nordic DApps Inc. — see the provenance section below.

## Status

Fork of the last Apache-licensed upstream commit (January 2024). It builds and runs, but the
chain has since moved to v3 messages that this version does not yet speak; until the
modernisation work lands, registration against the live network will not succeed. Track
progress in the issues.

## Build

```sh
make build            # ./bin/dvpnd  (needs Go ≥ 1.21, gcc for sqlite)
./bin/dvpnd --help
```

Configuration lives in `~/.dvpnd/config.toml` (`dvpnd config init`). Existing installs of
the upstream node keep their data in `~/.sentinelnode`; `dvpnd` does not move it — it logs
a hint and you copy the directory when you are ready.

## Provenance and license

This repository is a fork of [`sentinel-official/dvpn-node`](https://github.com/sentinel-official/dvpn-node)
(GitHub repository id 193683313, since renamed `sentinel-official/sentinel-dvpnx`), taken at commit
[`62bde16ac4ac1135a61ce3e254973f7567d4c593`](https://github.com/sentinel-official/sentinel-dvpnx/commit/62bde16ac4ac1135a61ce3e254973f7567d4c593)
of **2024-01-25** — the last commit upstream published under the **Apache License 2.0**.
The very next upstream commit (`99603eff`, 2024-03-31) replaced that license with a
non-open-source one. **No code, configuration or documentation from that commit or any later
upstream commit is included here.** The Apache 2.0 grant on everything before it is perpetual
and irrevocable (License §2), which is what makes this fork possible.

- License: Apache License 2.0, unchanged, with the original `Copyright [2017] [Sentinel]` notice
  retained — see [`LICENSE`](LICENSE) and [`NOTICE`](NOTICE).
- Verifiable record: [`docs/provenance/`](docs/provenance/) contains a script that re-derives the
  claims above from this repository, proxy.golang.org, sum.golang.org, apache.org and Software
  Heritage, plus its output at fork time.
- Contributing rules that keep the codebase clean: [`CONTRIBUTING.md`](CONTRIBUTING.md).

This project is not affiliated with, endorsed by or sponsored by Sentinel, the Sentinel dVPN
Foundation or Nordic DApps Inc. Running a VPN node is regulated or prohibited in some
jurisdictions; operators are responsible for complying with the law where they run it. The
software is provided "as is", without warranty, as the License states.
