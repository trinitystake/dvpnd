# Protocols: design note

What a protocol needs from the host, from Docker and from the node's code, for the six
protocols dvpnd runs. This is the record of the decisions taken on 2026-09-08, when the
four newest were added; `docs/operator.md` is what an operator reads. Update this file
whenever a protocol changes or a decision here does.

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
| 3 | `openvpn` | A | shipped; an OpenVPN client tunnelled through it between two containers over UDP and TCP, and a removed peer was killed and denied |
| 4 | `xray` | B | shipped; a VLESS client tunnelled through it on a test machine over TLS and over REALITY |
| 5 | `amneziawg` | A | shipped; an awg client tunnelled through it between two containers, with a signature packet set |
| 6 | `hysteria2` | B | shipped; the Hysteria client tunnelled through it on a test machine, with and without obfuscation |

The numbers and names are what client apps on the network use; they were taken from the
request/response handling of the maintainer's own client (see `CONTRIBUTING.md`), never from
the network's node software. The number is what `GET /status` reports and what aggregators
key on; the name is what `GET /` reports as `service_type`.

## The contract every service must satisfy

- `Info()`: the first two bytes are the listen port, big-endian; the rest is
  protocol-specific. The legacy session endpoint returns it raw to old clients.
- `GET /` answers the root document in the layout nodes on the network use (observed from
  a client): `{addr, uplink, downlink (bytes per second, as strings), handshake_dns,
  location{city, country, country_code, latitude, longitude}, moniker, peers,
  service_type, service_metadata, version{tag, commit}}`. Aggregators read `version.tag`;
  the legacy status fields (with `version` as a string) stay on `GET /status`.
- `service_metadata` in the root document is the service's `PublicMetadata()`: the keys of
  the handshake entry with everything per-session or secret blanked, per type as the
  network publishes it. wireguard `{port: 0, public_key: null}`; amneziawg the same plus
  `s1..s4, h1..h4: 0`; v2ray `{port: "", proxy_protocol, transport_protocol,
  transport_security, tls_pin: ""}`; xray the same plus `flow`, `method`, `key` and the
  `reality_*` keys, all blank; hysteria2 `{port: 0, tls_pin: "", obfs_password:
  "<redacted>" or ""}`; openvpn `{port: 0, protocol, ca: null, tls: null}`. Codes: proxy
  VLESS = 1, VMess = 2; transport tcp = 1; security none = 1, tls = 2, reality = 3; flow
  none = 1, vision = 2.
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
certificate pin, a self-signed certificate generator, the generic config CLI built from a
`ConfigSpec`, a child-process helper (start, reap, SIGTERM then kill), a peer set, and the
NAT and forwarding rule set for tunnel interfaces. The WireGuard service exports its
uplink detection and forwarding switch for the other tunnel protocols.

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

### AmneziaWG (shipped)

The WireGuard service parameterised by a `Variant` (tool names `awg`/`awg-quick`, interface
`awg0`, configuration directory `/etc/amnezia/amneziawg`) plus extra `[Interface]` lines;
`services/amneziawg` reads `amneziawg.toml` (WireGuard's keys and an `[obfuscation]`
section generated at `config init`), hands the WireGuard part and the parameter lines to
the core, and adds `s1`–`s4`, `h1`–`h4` and `i1`–`i5` to the handshake metadata entry.
Junk packet counts (`jc`, `jmin`, `jmax`) are per side and not sent; the node sends few and
small ones, `s3` and `s4` stay 0 to keep the tunnel MTU. Validation mirrors what clients
check: paddings within a datagram, `s1 + 56 != s2`, headers all distinct and above 4 (or all
zero). The public listing carries the port only.

Versions follow the client apps: amneziawg-go at commit `1cc9427` (tag v0.2.19) and
amneziawg-tools v1.0.20260618-2, both built from source in the image; on a host the
operator installs the tools and either the DKMS kernel module or amneziawg-go. In Docker
only the userspace implementation is possible (`--device /dev/net/tun`, no SYS_MODULE).

### OpenVPN (shipped)

The node is its own certificate authority (`services/openvpn/pki.go`): an ECDSA P-256 CA,
server certificate and a 2048-bit tls-crypt key created on the first start and kept under
`<home>/openvpn/`; every `AddPeer` issues a client certificate (common name = the peer's
UUID) and PKCS#8 key, returned in the handshake with the CA and the tls-crypt key in the
shape the client apps assemble a profile from. The server configuration matches the
profile's pins (ECDSA TLS cipher, AES-GCM data channel, SHA256, `remote-cert-tls`).

Admission is `management-client-auth` with `auth-user-pass-optional`: the node keeps one
connection to the management interface (`services/openvpn/management.go`), answers every
`>CLIENT:CONNECT` with `client-auth-nt` for a registered common name or `client-deny`
otherwise, reads per-client bytes from `status 3` (received = the client's upload, sent =
its download), banks a connection's final counters from `>CLIENT:DISCONNECT` so a reconnect
does not lose usage, and removes a peer with `client-kill <id> RESTART,…`, which makes the
client reconnect at once and be denied. NAT and forwarding use the same rule set as
WireGuard (`services/common/nat.go`), installed by the node around the server's lifetime.

Binary: `openvpn` from apk (2.7 in the image) and apt. The management port is a loopback TCP
port without a password, as OpenVPN warns at start: on a dedicated node the only other user
of loopback is root already.

## Packaging policy

One image. Each protocol's binary is added when the protocol lands, pinned to a release and
checked against a sha256, like the hnsd commit check in the Dockerfile. Server versions
follow the versions the maintainer's client bundles, so a node and a client are tested
against the same code. On a host the operator installs the same binary; §6 of the operator
guide lists each one.
