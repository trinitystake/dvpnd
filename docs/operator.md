# Running a dvpnd node

What you need: a Linux host (Ubuntu 22.04/24.04 or Debian 12/13, x86_64) with a **public IPv4
address** (a home connection behind carrier-grade NAT will not work — check that your
router's WAN address is not in 100.64.0.0/10), root access, and an account on the Sentinel
chain holding a little DVPN for gas. Registration deposit is currently 0.

This guide covers WireGuard, V2Ray and XRAY nodes; the protocol is chosen with `[node] type`.
A WireGuard node has been run this way against the public network with real clients; an XRAY
node served a VLESS client end to end on a test machine, over TLS and over REALITY; the V2Ray
path is documented from the code and has not been exercised end to end yet.

## Which way to run it

Run the node on the host as a systemd service (§6), on a VPS that does nothing else. Use
Docker (§7) only when the host path does not fit: you want the bundled `v2ray` and `hnsd`
without installing them, you cannot put a Go toolchain on the host, or everything on that
host already runs in Docker.

The reason is where the protocol's data plane lives. A **WireGuard** node creates a tunnel
interface on the host and NATs the peers' traffic out of the uplink: on the host that is one
NAT hop with native IPv6; in a container it is two NAT hops, IPv6 needs a Docker daemon
change, and the container has to be given NET_ADMIN, SYS_MODULE and the host's kernel modules
anyway, so the isolation is nominal. A **V2Ray** or **XRAY** node is a userspace proxy on one TCP
port with no privileges at all: host and Docker are equal, and the host wins only for having
one way of running things across node types.

Whichever way: a dedicated machine with its own IP. The node runs as root, keeps an
unencrypted key, and routes strangers' traffic out of that IP, so abuse complaints land there.
Never on a validator host, never next to anything with secrets.

## 1. Build

Build on the host (cgo is needed for sqlite, so cross-compiling is awkward; there is no
binary release yet):

```sh
sudo apt-get update && sudo apt-get install -y git build-essential
# Go 1.26 or newer: https://go.dev/dl/  (apt's may be too old)
git clone <this repository> dvpnd && cd dvpnd      # or copy the source tree over
make build && sudo install -m 0755 bin/dvpnd /usr/local/bin/dvpnd
dvpnd version
```

## 2. Configuration

```sh
dvpnd config init                     # writes ~/.dvpnd/config.toml (run as root: the node runs as root)
```

Edit `~/.dvpnd/config.toml`:

| Key | Set to |
|---|---|
| `[keyring] backend` | `test` — the node must sign transactions unattended, so the key is stored unencrypted under `~/.dvpnd/keyring-test` (mode 700). Use a **dedicated operator key** and sweep earnings out regularly; see §8. |
| `[keyring] from` | the key name you will create in step 3, e.g. `operator` |
| `[node] type` | `wireguard`, `v2ray` or `xray`; see §2a, §2b, §2c for the protocol's own file |
| `[node] moniker` | your node's public name (4–32 characters) |
| `[node] gigabyte_prices` | e.g. `40000000udvpn` (40 DVPN per GB). Only denoms the chain lists in its node params are accepted; prices below the chain's minimums are rejected at registration. |
| `[node] hourly_prices` | e.g. `97500000udvpn` |
| `[node] remote_url` | `https://<public-ip>:8585` — this becomes the on-chain `remote_addrs` (`host:port`); clients connect to it directly |
| `[node] listen_on` | `0.0.0.0:8585` |
| `[handshake] enable` | `false` unless you install `hnsd` (Handshake DNS resolver); must be `false` on a proxy node (V2Ray, XRAY) |
| `[geoip]` | Leave `provider = "auto"`: at start the node asks ipwho.is, then ip2location.io, for the location of its public IP and cross-checks the country against Cloudflare; a disagreement is logged. No key, no cost, and both services allow commercial use on their free tier. Check the result with `curl -sk https://127.0.0.1:8585/status \| jq .result.location` (`source` names the service that answered). The location a node reports is self-declared and nothing verifies it; clients use it to choose a node, so only if the lookup is wrong set `city`, `country` (name or ISO code), `latitude`, `longitude` to the server's real physical location. The node logs the contradiction and reports `source = "static"`. Do not use `ip-api` on a node that earns unless you pay for it: its free tier is non-commercial only (a paid key goes in `url`). `ipinfo` returns the country only on its free plan. |
| `[chain] rpc_addresses` | comma-separated, tried in order; the defaults are public endpoints from the [chain registry](https://github.com/cosmos/chain-registry/blob/master/sentinel/chain.json). Put your own RPC first if you run one. An endpoint that answers with an HTTP redirect does not work with this client. |

Geolocation services: ipwho.is and Cloudflare require no attribution. dvpnd uses IP2Location.io
IP geolocation web service.

### 2a. WireGuard

```sh
dvpnd wireguard config init           # writes ~/.dvpnd/wireguard.toml
```

Pick a fixed `listen_port` (the default is random) and keep it — it is what clients are told
to connect to. `uplink` may stay empty; the interface of the default route is detected at
start and used for NAT. `enable_ipv6` is `true`, as on every other node: the host must then
reach the IPv6 internet (check in §6; Docker needs extra steps, §7). Set it `false` for an
IPv4-only tunnel; clients then exit with the node's IPv4 address only.

### 2b. V2Ray

```sh
dvpnd v2ray config init               # writes ~/.dvpnd/v2ray.toml
```

Pick a fixed `listen_port` (TCP). `transport = "tcp"` is the only transport confirmed with
current client apps. `tls = true` wraps the VMess inbound in TLS using the node's
`tls.crt`/`tls.key` from §4 and advertises the certificate's pin to clients; `false` relies
on VMess's own encryption. The node needs the `v2ray` binary on `PATH` (see §6) and drives it
over loopback port 23, so nothing else on the host may bind that port.

### 2c. XRAY

```sh
dvpnd xray config init                # writes ~/.dvpnd/xray.toml with a fresh REALITY key pair
```

One VLESS inbound on `[vless] listen_port` (TCP; pick a fixed one). `security` chooses how
it is wrapped:

- `tls` (default): the node's `tls.crt`/`tls.key` from §4; clients receive the certificate's
  pin in the handshake and connect to nothing else.
- `reality`: no certificate. The node imitates the TLS handshake of `[reality] server_name`,
  so to an observer the port looks like that site. The site must serve TLS 1.3 with HTTP/2
  on port 443 and answer with a small certificate chain: `www.apple.com` (the default) and
  `www.cloudflare.com` work with current xray, `www.microsoft.com` does not. The key pair
  and `short_id` are generated by `config init`; clients receive the public key, short id,
  server name and `fingerprint` in the handshake. Only change the keys with a fresh
  `config init --force`.

`flow = true` enables XTLS Vision, which current clients support and expect. The node drives
xray over loopback `[api] port`; nothing else may bind it. Destinations in private and
loopback ranges are blocked for clients, so a client cannot reach that port or anything else
on the host through the proxy. The node needs the `xray` binary on `PATH` (see §6).

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

| Node type | Port | Protocol |
|---|---|---|
| all | `[node] listen_on`, 8585 above | tcp |
| wireguard | `listen_port` in `wireguard.toml` | udp |
| v2ray | `listen_port` in `v2ray.toml` | tcp |
| xray | `listen_port` in `xray.toml` | tcp |

```sh
sudo ufw allow OpenSSH && sudo ufw allow 8585/tcp && sudo ufw allow <listen_port>/udp && sudo ufw enable
```

WireGuard only: peer traffic is NAT-ed through the uplink interface. The node enables
`net.ipv4.ip_forward` and `net.ipv6.conf.all.forwarding` itself when it brings the interface
up, and it accepts established replies back into the tunnel itself, so a FORWARD policy of
DROP (ufw's default, or a host where Docker is or was installed) is fine. Ports published by
Docker bypass ufw: on a Docker host the ufw rules above only protect what is not published.

## 6. Run as a service (recommended)

Before the first start, install what the protocol needs on the host:

- **WireGuard:** `sudo apt-get install -y wireguard-tools`. Every kernel since 5.6 has the
  module. With `enable_ipv6 = true` the host must reach the IPv6 internet:
  `ping -6 -c1 2606:4700:4700::1111` from the host must answer, otherwise set it `false`.
  No sysctl or daemon change: the node turns forwarding on at every start.
- **V2Ray:** the `v2ray` binary (v5) on `PATH`. Download `v2ray-linux-64.zip` from the
  [v2fly/v2ray-core releases](https://github.com/v2fly/v2ray-core/releases), unzip it and
  `sudo install -m 0755 v2ray /usr/local/bin/v2ray`; `v2ray version` must work. The Docker
  image bundles Alpine's package; `docker run --rm dvpnd v2ray version` shows that version if
  you want to match it. The node refuses to start when the binary is missing.
- **XRAY:** the `xray` binary on `PATH`, release 26.3.27, the version current client apps
  bundle and the one the Docker image pins. Download `Xray-linux-64.zip` from the
  [XTLS/Xray-core release](https://github.com/XTLS/Xray-core/releases/tag/v26.3.27),
  check it against the `.dgst` file published next to it, unzip only the binary and
  `sudo install -m 0755 xray /usr/local/bin/xray`; `xray version` must work. The node
  refuses to start when the binary is missing.

Then:

```sh
sudo cp scripts/dvpnd.service /etc/systemd/system/dvpnd.service
sudo systemctl daemon-reload && sudo systemctl enable --now dvpnd
journalctl -u dvpnd -f
```

First start: the node runs a speed test (about a minute), registers (`MsgRegisterNode`),
marks itself active (`MsgUpdateNodeStatus`), starts its service (`wg0` up, or the proxy as a
child process) and serves `https://<ip>:8585`. Check it:

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

`systemctl stop` or `restart` sends SIGTERM; the node stops its service first (tunnel down
and NAT rules removed, or the proxy child exited) and exits. Connected peers are dropped
and reconnect on their own; the session database is kept and reconciled with the chain at
the next start.

## 7. Run with Docker (when the host path does not fit)

Use this when you want the bundled `v2ray`, `xray` and `hnsd`, cannot install Go on the
host, or the host already runs everything in Docker. Build the image (the Dockerfile uses BuildKit cache
mounts, so BuildKit must be on — it is by default on current Docker; otherwise prefix the
command with `DOCKER_BUILDKIT=1`):

```sh
make build-image                    # docker build ... --tag dvpnd
```

**IPv6 (WireGuard nodes).** The tunnel is dual-stack by default (`enable_ipv6 = true` in
`wireguard.toml`), so the node must reach the IPv6 internet, otherwise every IPv6 connection
a client opens through the tunnel is answered with "unreachable"; browsers fall back to IPv4
but other software may fail. On Docker's default bridge a container cannot reach IPv6, so
either set `enable_ipv6 = false` for an IPv4-only tunnel, or give containers IPv6 with
`/etc/docker/daemon.json` and a Docker restart (a running container restarts with it):

```json
{
  "ipv6": true,
  "fixed-cidr-v6": "fd00:d0c:1::/64",
  "ip6tables": true
}
```

```sh
sudo systemctl restart docker
docker run --rm alpine ping -6 -c1 2606:4700:4700::1111   # optional: a reply means containers reach IPv6
```

Docker enables IPv6 forwarding on the host itself, so no sysctl change is needed. If the
check fails and the host has no IPv6 connectivity at all, set `enable_ipv6 = false` instead.
Docker NATs the container's IPv6 to the host's address, so with IPv6 on, clients exit with
the host's IPv6 address on IPv6-capable sites. A V2Ray container without IPv6 only fails
for IPv6-only destinations, which are rare. `scripts/runner.sh setup` writes its own
`daemon.json` and overwrites an existing one.

**Configuration.** The image is named `dvpnd` and its entrypoint binary is `process`; the
command after the image name is passed to the node. Prepare `config.toml`, the protocol's
file, the key and `tls.crt`/`tls.key` in `/root/.dvpnd` exactly as in sections 2 to 4 (run
the host binary, or `docker run --rm -v /root/.dvpnd:/root/.dvpnd dvpnd process config init`).
The container runs as root, so those files must be root-owned.

**WireGuard node:**

```sh
docker run --detach --name dvpnd --restart unless-stopped \
  --log-opt max-size=50m --log-opt max-file=3 \
  --volume /lib/modules:/lib/modules:ro \
  --volume /root/.dvpnd:/root/.dvpnd \
  --cap-drop ALL \
  --cap-add NET_ADMIN --cap-add NET_BIND_SERVICE --cap-add NET_RAW --cap-add SYS_MODULE \
  --sysctl net.ipv4.ip_forward=1 \
  --sysctl net.ipv6.conf.all.disable_ipv6=0 \
  --sysctl net.ipv6.conf.all.forwarding=1 \
  --sysctl net.ipv6.conf.default.forwarding=1 \
  --publish 8585:8585/tcp \
  --publish <listen_port>:<listen_port>/udp \
  dvpnd process start
```

The container drops every capability except the four WireGuard needs. IP forwarding is passed
in with `--sysctl` because `/proc/sys` is read-only inside an unprivileged container: the node
reads those switches and, finding them already on, leaves them alone. Publish the same UDP
port as `wireguard.toml`'s `listen_port`.

**V2Ray node:**

```sh
docker run --detach --name dvpnd --restart unless-stopped \
  --log-opt max-size=50m --log-opt max-file=3 \
  --volume /root/.dvpnd:/root/.dvpnd \
  --cap-drop ALL --cap-add NET_BIND_SERVICE \
  --publish 8585:8585/tcp \
  --publish <listen_port>:<listen_port>/tcp \
  dvpnd process start
```

No modules, no sysctls, no NET_ADMIN: the proxy is a plain process on a port. Publish the
same TCP port as `v2ray.toml`'s `listen_port`.

**XRAY node:** the same command with `xray.toml`'s `listen_port`. The image pins xray
26.3.27 and checks its sha256 at build time.

The `--log-opt` flags cap the container's log at three files of 50 MB; Docker's default
json-file log grows without bound. `scripts/runner.sh` wraps these commands (`init`, `start`,
`stop`, `status`, `update`) for every node type; edit its `NODE_IMAGE` if you push the image
to a registry.

The WireGuard path is exercised by an end-to-end test that builds the image, registers a
node, buys a session and connects a client from a second container — a WireGuard node served
real traffic this way and reported it on chain.

## 8. Operating

- **Logs:** `journalctl -u dvpnd` on the host, `docker logs dvpnd` in Docker. Every
  transaction logs its hash and code.
- **Earnings** accrue to the operator `sent1…` address as sessions settle. Sweep them to a
  wallet you hold offline; the key on the node is unencrypted.
- **Upgrade:** on the host, build the new version, `sudo systemctl stop dvpnd`, install the
  binary, `sudo systemctl start dvpnd`. In Docker, rebuild or pull the image, `docker rm -f
  dvpnd` and rerun the run command. Existing peers are dropped on restart; the local session
  database (`data.db`) is kept and reconciled with the chain.
- **Moving hosts:** copy `~/.dvpnd` (keyring, config, TLS, protocol file) to the new host and
  change `remote_url`; the next start sends `MsgUpdateNodeDetails` with the new address.
- **Stopping for good:** `sudo systemctl disable --now dvpnd`. The chain marks the node
  inactive after `status_timeout` (currently 1 h) without an update; to do it immediately run
  `go run ./tools/e2e -home /root/.dvpnd -deactivate` from the source tree.
- **Upstream node data:** if this host ran the upstream node, its files are in
  `~/.sentinelnode`; `dvpnd` never reads them but prints a hint when only that directory exists.
