# Sentinel dVPN Node

[![CodeQL](https://github.com/sentinel-official/dvpn-node/actions/workflows/codeql.yml/badge.svg)](https://github.com/sentinel-official/dvpn-node/actions/workflows/codeql.yml)
[![Docker](https://github.com/sentinel-official/dvpn-node/actions/workflows/docker-publish.yml/badge.svg)](https://github.com/sentinel-official/dvpn-node/actions/workflows/docker-publish.yml)
[![Go](https://img.shields.io/github/go-mod/go-version/sentinel-official/dvpn-node)]()
[![GoReport](https://goreportcard.com/badge/github.com/sentinel-official/dvpn-node)](https://goreportcard.com/report/github.com/sentinel-official/dvpn-node)
[![Licence](https://img.shields.io/github/license/sentinel-official/dvpn-node.svg)](https://github.com/sentinel-official/dvpn-node/blob/master/LICENSE)
[![Tag](https://img.shields.io/github/tag/sentinel-official/dvpn-node.svg)](https://github.com/sentinel-official/dvpn-node/releases/latest)
[![TotalLines](https://img.shields.io/tokei/lines/github/sentinel-official/dvpn-node)]()

For documentation click [here](https://docs.sentinel.co/node-setup)

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
