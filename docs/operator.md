# Running a dvpnd node

What you need: a Linux host (Ubuntu 22.04/24.04 or Debian 12, x86_64) with a **public IPv4
address** (a home connection behind carrier-grade NAT will not work — check that your
router's WAN address is not in 100.64.0.0/10), root access, and an account on the Sentinel
chain holding a little DVPN for gas. Registration deposit is currently 0.

This guide sets up a WireGuard node. V2Ray nodes need the `v2ray` binary as well and are not
covered yet.

## 1. Build

Build on the host (cgo is needed for sqlite, so cross-compiling is awkward):

```sh
sudo apt-get update && sudo apt-get install -y git build-essential wireguard-tools
# Go 1.23 or newer: https://go.dev/dl/  (apt's may be too old)
git clone <this repository> dvpnd && cd dvpnd      # or copy the source tree over
make build && sudo install -m 0755 bin/dvpnd /usr/local/bin/dvpnd
dvpnd version
```

## 2. Configuration

```sh
dvpnd config init                     # writes ~/.dvpnd/config.toml (run as root: the node runs as root)
dvpnd wireguard config init           # writes ~/.dvpnd/wireguard.toml
```

Edit `~/.dvpnd/config.toml`:

| Key | Set to |
|---|---|
| `[keyring] backend` | `test` — the node must sign transactions unattended, so the key is stored unencrypted under `~/.dvpnd/keyring-test` (mode 700). Use a **dedicated operator key** and sweep earnings out regularly; see §7. |
| `[keyring] from` | the key name you will create in step 3, e.g. `operator` |
| `[node] moniker` | your node's public name (4–32 characters) |
| `[node] gigabyte_prices` | e.g. `40000000udvpn` (40 DVPN per GB). Only denoms the chain lists in its node params are accepted; prices below the chain's minimums are rejected at registration. |
| `[node] hourly_prices` | e.g. `97500000udvpn` |
| `[node] remote_url` | `https://<public-ip>:8585` — this becomes the on-chain `remote_addrs` (`host:port`); clients connect to it directly |
| `[node] listen_on` | `0.0.0.0:8585` |
| `[node] type` | `wireguard` |
| `[handshake] enable` | `false` unless you install `hnsd` (Handshake DNS resolver) |
| `[geoip]` | `provider = "ipify"` gives only the IP; set `city`, `country`, `latitude`, `longitude` by hand so clients see the right location. Do not use `ip-api` for a node that earns: its free tier is non-commercial only. |
| `[chain] rpc_addresses` | keep the defaults or add your own RPC; several, comma-separated, are tried in order |

`~/.dvpnd/wireguard.toml`: pick a fixed `listen_port` (the default is random) and keep it — it
is what clients are told to connect to. `uplink` may stay empty; the interface of the default
route is detected at start and used for NAT.

## 3. Key

```sh
dvpnd keys add operator                # prints the mnemonic once: store it safely
dvpnd keys add operator --recover      # or import an existing mnemonic
dvpnd keys list                        # shows the sent1… operator and sentnode1… node address
```

Send a few DVPN to the `sent1…` address for gas (each status update and usage report is a
transaction; budget roughly 0.02 DVPN per transaction, a status update every 48 minutes).

## 4. TLS certificate

Clients pin the certificate per session, so a self-signed one is what the network expects:

```sh
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 -nodes -days 3650 \
  -subj "/CN=dvpnd" -keyout ~/.dvpnd/tls.key -out ~/.dvpnd/tls.crt
```

## 5. Firewall and forwarding

```sh
sudo ufw allow OpenSSH && sudo ufw allow 8585/tcp && sudo ufw allow <listen_port>/udp && sudo ufw enable
```

Peer traffic is NAT-ed through the uplink interface; the node enables
`net.ipv4.ip_forward` and `net.ipv6.conf.all.forwarding` itself when it brings the
WireGuard interface up.

## 6. Run as a service

```sh
sudo cp scripts/dvpnd.service /etc/systemd/system/dvpnd.service
sudo systemctl daemon-reload && sudo systemctl enable --now dvpnd
journalctl -u dvpnd -f
```

First start: the node runs a speed test (about a minute), registers (`MsgRegisterNode`),
marks itself active (`MsgUpdateNodeStatus`), brings up `wg0` and serves `https://<ip>:8585`.
Check it:

```sh
curl -sk https://127.0.0.1:8585/ | head -c 400            # {"success":true,"result":{"service_type":"wireguard",…}}
```

On the chain, the node appears in `sentinel/node/v3/nodes/<sentnode1…>` with status
`active`. Node aggregators that client apps read re-probe active nodes every few minutes;
once your `GET /` answers from the internet your node becomes visible in the apps.

The node adapts its cadence to the chain: it never lets `interval_update_status` or
`interval_update_sessions` exceed 80% of the chain's `status_timeout` parameters (a silent
node is deactivated by the chain after that timeout; a session that reports nothing is
cancelled).

## 7. Operating

- **Logs:** `journalctl -u dvpnd`. Every transaction logs its hash and code.
- **Earnings** accrue to the operator `sent1…` address as sessions settle. Sweep them to a
  wallet you hold offline; the key on the node is unencrypted.
- **Upgrade:** build the new version, `sudo systemctl stop dvpnd`, install the binary,
  `sudo systemctl start dvpnd`. Existing peers are dropped on restart; the local session
  database (`data.db`) is kept and reconciled with the chain.
- **Moving hosts:** copy `~/.dvpnd` (keyring, config, TLS, WireGuard key) to the new host and
  change `remote_url`; the next start sends `MsgUpdateNodeDetails` with the new address.
- **Stopping for good:** `sudo systemctl disable --now dvpnd`. The chain marks the node
  inactive after `status_timeout` (currently 1 h) without an update; to do it immediately run
  `go run ./tools/e2e -home /root/.dvpnd -deactivate` from the source tree.
- **Upstream node data:** if this host ran the upstream node, its files are in
  `~/.sentinelnode`; `dvpnd` never reads them but prints a hint when only that directory exists.
