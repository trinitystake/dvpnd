# Protocols: design note

What a protocol needs from the host, from Docker and from the node's code, for the two
protocols dvpnd runs today and the four planned. This is the record of the decisions taken
on 2026-09-08; `docs/operator.md` is what an operator reads. Update this file whenever a
protocol lands or a decision here changes.

## Two classes

| Class | Protocols | Data plane | Host path | Docker path |
|---|---|---|---|---|
| **A: tunnel + NAT on the host** | WireGuard, AmneziaWG, OpenVPN | kernel module or tun device, `ip_forward`, MASQUERADE | recommended: one NAT hop, native IPv6, kernel data plane | NET_ADMIN, NET_RAW (+SYS_MODULE and `/lib/modules` for kernel modules, `--device /dev/net/tun` for tun), sysctls, a second NAT hop, `daemon.json` for IPv6; kernel modules cannot ship in an image |
| **B: userspace proxy on a port** | V2Ray, XRAY, Hysteria2 | a child process listening on one TCP/UDP port | equal: the binary on `PATH`, no privileges | equal: `--cap-drop ALL`, one published port |

One protocol per node process, chosen with `[node] type`. A node that should offer two
protocols is two nodes. Running several protocols in one process would touch the status
API, the handshake, the session table and every client; that is a separate decision.

## Status

| Type | Name | Class | State |
|---|---|---|---|
| 1 | `wireguard` | A | shipped, exercised against the public network |
| 2 | `v2ray` | B | shipped, not yet exercised end to end |
| 3 | `openvpn` | A | planned (phase 5) |
| 4 | `xray` | B | shipped; a VLESS client tunnelled through it on a test machine over TLS and over REALITY |
| 5 | `amneziawg` | A | planned (phase 4) |
| 6 | `hysteria2` | B | shipped; the Hysteria client tunnelled through it on a test machine, with and without obfuscation |

The numbers and names are what client apps on the network use; they were taken from the
request/response handling of the maintainer's own client (see `CONTRIBUTING.md`), never from
the network's node software. The number is what `GET /status` reports and what aggregators
key on; the name is what `GET /` reports as `service_type`.

## The contract every service must satisfy

- `Info()`: the first two bytes are the listen port, big-endian; the rest is
  protocol-specific. The legacy session endpoint returns it raw to old clients.
- `GET /` answers `{service_type, service_metadata: [{port, proxy_protocol,
  transport_protocol, transport_security, tls_pin?}]}`. Codes: proxy VLESS = 1, VMess = 2;
  transport tcp = 1; security none = 1, tls = 2, reality = 3.
- `POST /` (handshake): body `{data: base64(JSON peer request), id, pub_key:
  "secp256k1:…", signature}`; the signature covers `BE64(id) || raw JSON`. The answer is
  `{success: true, result: {data: base64(JSON payload), addrs: [node hosts]}}`.
- The session key stored by the node is `base64(peer data)`; `Peers()` must return each
  peer with exactly that key, or its usage is never reported.
- `RemovePeer` is called by the node (session expired, allocation exceeded, account
  evicted), never by the client.

Per type:

| Type | Peer request | Peer data | Payload in `result.data` |
|---|---|---|---|
| 1 wireguard | `{public_key}` base64, 32 bytes | the 32 bytes | `{addrs: ["10.8.0.x/32", "…/128"], metadata: [{port, public_key}]}` |
| 2 v2ray | `{uuid}` 16-byte array or canonical string | `0x01` + 16 bytes | `{metadata: [inbound with tls_pin]}` |
| 4 xray | `{uuid}` 16-byte array | proxy byte + 16 bytes | `{metadata: [{port, proxy_protocol: 1, transport_protocol: 1, transport_security: 2 or 3, tls_pin (hex sha256 of the cert), flow?: 2 (xtls-rprx-vision), reality_server_name, reality_short_id, reality_public_key (32-byte x25519, base64), reality_fingerprint}]}`; clients refuse an all-cleartext list |
| 6 hysteria2 | `{uuid}` canonical string (accept a 16-byte array too) | the 16 bytes | `{metadata: [{port, tls_pin (64 hex chars, mandatory), obfs_password}]}`; the client authenticates with the uuid string |
| 5 amneziawg | `{public_key}` base64, 32 bytes | the 32 bytes | `{addrs, metadata: [{port, public_key, s1, s2, s3?, s4?, h1, h2, h3, h4, i1..i5?}]}`; junk-packet counts (Jc/Jmin/Jmax) are per side, S/H must match the server, I1–I5 are forwarded when present |
| 3 openvpn | `{uuid}` 16-byte array | the 16 bytes | `{metadata: [{port, protocol: "udp" or "tcp", ca: base64 DER, tls: base64 256-byte tls-crypt key}], cert: base64 DER client certificate, key: base64 DER PKCS#8}`; the host comes from the top-level `addrs` |

## Adding a protocol

A protocol is one package under `services/<name>/` that implements `types.Service` (the
data-plane methods plus `Name`, `ParsePeerRequest`, `HandshakePayload` and `Metadata`), and
one entry in the list in `services/registry.go` (name, type number, constructor, CLI subtree,
whether Handshake DNS may run next to it). Everything else goes through the registry: the
`[node] type` validation, the `dvpnd <name> config init|show|set` commands, the root
document's `service_type` and `service_metadata`, and both halves of the handshake.
`services/common/` has what protocols share: UUID parsing for peer requests, the TLS
certificate pin, and the generic config CLI built from a `ConfigSpec`.

Outside the code, a protocol also needs a `scripts/runner.sh` branch, its binary in the
`Dockerfile`, and its sections in `docs/operator.md` (§2x, §5, §6, §7).

Every service ships with unit tests that never touch the network (fake binaries on `PATH`,
loopback fakes for control channels, no root), the binary pinned with a checksum in the
image, and a real session from a client, described in words only in the README.

## Per-protocol design

### WireGuard (shipped)

`wg-quick` brings `wg0` up from a rendered config; PostUp adds the FORWARD accept rules (both
directions) and MASQUERADE on the uplink; peers via `wg set … allowed-ips …`; usage from
`wg show … transfer`. IPv4 and optional IPv6 pools hand out tunnel addresses.

### V2Ray (shipped)

`v2ray run --config <json>` as a child process; VMess inbound on `listen_port`, optional TLS
with the node's certificate (pin advertised); peers and usage over the gRPC control API on
loopback. The control port is hard-coded to 23.

### XRAY (shipped)

Generalised from V2Ray: the same JSON shape with a VLESS inbound over raw TCP,
`security = "tls"` (node certificate, pin advertised) or `"reality"` (x25519 key pair and
short id generated at `config init`, `server_name` chosen by the operator, default
`www.apple.com`), XTLS Vision on by default, control over xray's gRPC API on a configurable
loopback port. Private and loopback destinations are blocked by explicit CIDRs (not
`geoip:private`, which needs the asset file), so a client cannot reach the control port.

The four API messages the node sends (`AlterInbound` with `AddUserOperation` /
`RemoveUserOperation`, `QueryStats`) are encoded by hand with `protowire` in
`services/xray/wire.go`: xray-core and v2ray-core register the same proto file paths, so
both cannot be linked into one binary. xray-core stays a test-only dependency whose real
generated code decodes what the node encodes. Binary: Xray release 26.3.27, sha256 checked
in the Dockerfile.

Found while testing against the real binary: REALITY with `www.microsoft.com` as the
imitated site fails the handshake with xray 26.3.27 (the site's certificate chain is too
large for the handshake replay); `www.apple.com` and `www.cloudflare.com` work.

### Hysteria2 (shipped)

`hysteria server -c <yaml>`: one UDP port, TLS with the node's certificate (the pin is
mandatory for clients), optional Salamander obfuscation password. The node serves the
server's HTTP authentication hook on loopback and answers `{ok, id}` for a registered peer,
where the password is the peer's canonical UUID and the id is the session key, so the
statistics API files traffic under the session key directly. Usage comes from
`GET /traffic?clear=1` (deltas, accumulated by the node; the server counts from the
client's point of view: `tx` is the client's upload, `rx` its download, checked with a large
download), removal is `POST /kick`. The node raises `net.core.rmem_max`/`wmem_max` on a host
(not namespaced, so in Docker it is a host setting). Binary: release app/v2.10.0, sha256
checked in the Dockerfile. Port hopping is out of scope.

### AmneziaWG (phase 4)

WireGuard's implementation parameterised by tool names (`awg`, `awg-quick`), interface name
(`awg0`) and the obfuscation keys in `[Interface]`, generated once at `config init`. Host:
`amneziawg-tools` plus the DKMS kernel module or `amneziawg-go` on a tun; Docker: userspace
only. The image builds `amneziawg-go` and `amneziawg-tools` from pinned tags.

### OpenVPN (phase 5)

The node generates its own PKI (ECDSA P-256 CA and server certificate, a tls-crypt key) and
per-session client certificates; `openvpn --config <file>` with `dev tun`, `management` on
loopback and `management-client-auth`, so the node approves each connecting common name
against its registered peers, reads per-client bytes from `status 3`, and kills a client on
removal. NAT and forwarding through the same helpers WireGuard uses. Binary: `openvpn` from
apk and apt.

## Packaging policy

One image. Each protocol's binary is added when the protocol lands, pinned to a release and
checked against a sha256, like the hnsd commit check in the Dockerfile. Server versions
follow the versions the maintainer's client bundles, so a node and a client are tested
against the same code. On a host the operator installs the same binary; §6 of the operator
guide lists each one.
