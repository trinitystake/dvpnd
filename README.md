# dvpnd

[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/trinitystake/dvpnd)](go.mod)

`dvpnd` is an open source dVPN node daemon for the Sentinel blockchain (chain ID
`sentinelhub-2`). It registers the node on-chain, serves WireGuard or V2Ray sessions to
subscribers, and reports usage. It is licensed under the Apache License 2.0 and is not
affiliated with Sentinel or Nordic DApps Inc. — see the provenance section below.

## Status

Speaks the current chain protocol: `sentinelhub` v12 message set (node/session/subscription
v3) on cosmos-sdk v0.47, implemented against the Apache-licensed chain sources only; see
`CONTRIBUTING.md` for the rules that keep it that way.

Verified end-to-end on the live network (`sentinelhub-2`) with a WireGuard node behind NAT
and the client side driven by `tools/e2e`, which handshakes the way current client apps do:
node registration, status keep-alive, session purchase, the `GET /` and `POST /` handshake,
tunnel traffic both ways, and usage reporting that the chain recorded.

Not yet exercised: V2Ray nodes, hourly and plan-subscription sessions, a node reachable from
the public internet, and a connection from a stock client app (`lite/live_test.go` covers
read-only queries, opt-in via `DVPND_LIVE_RPC`).

The node adapts its update cadence to the chain: it never lets `interval_update_status` or
`interval_update_sessions` exceed 80% of the chain's `status_timeout` parameters (currently
1 h and 2 h), because the chain deactivates a silent node and cancels a silent session.

## Node API

Clients talk to the node over HTTPS on `remote_url` (self-signed certificate; clients pin it).

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/` | Node document: `service_type` (`wireguard`/`v2ray`), `service_metadata` (inbounds: `port`, `proxy_protocol`, `transport_protocol`, `transport_security`) and the fields of `/status`. |
| `POST` | `/` | Handshake used by current client apps. Body `{data, id, pub_key, signature}`: `data` is the base64 peer request (`{"public_key"}` for WireGuard, `{"uuid"}` for V2Ray), `pub_key` is `secp256k1:` + base64 compressed key, `signature` the compact signature over the 8-byte big-endian session id followed by the raw `data` bytes. Returns `{result:{data, addrs}}` with `data` = base64 JSON of the client configuration (WireGuard: `addrs` assigned to the client and `metadata[{port, public_key}]`; V2Ray: `metadata[{port, proxy_protocol, transport_protocol, transport_security, tls_pin}]`). |
| `GET` | `/status` | Legacy status document. |
| `POST` | `/accounts/:acc_address/sessions/:id` | Legacy handshake (`{key, signature}`, signature over the session id, verified against the account's on-chain public key). |

Errors are `{success:false, error:{code, message}}`; a session or key that already
exists answers `409`.

## Running a node

See [`docs/operator.md`](docs/operator.md): host requirements (a public IPv4 — not behind
carrier-grade NAT), build, configuration, TLS, firewall, the systemd unit in
[`scripts/dvpnd.service`](scripts/dvpnd.service), and day-to-day operation.

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
