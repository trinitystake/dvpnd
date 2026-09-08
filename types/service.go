// SPDX-License-Identifier: Apache-2.0
// Modified from sentinel-official/dvpn-node @ 62bde16 (2024-01-25). See NOTICE.

package types

// Service is one VPN protocol as the node drives it. The first group of
// methods manages the data plane; the second speaks the client handshake
// documented in docs/protocols.md. Info()'s first two bytes must be the
// listen port, big-endian.
type Service interface {
	Type() uint64
	Name() string
	Info() []byte
	Init(home string) error
	Start() error
	Stop() error
	AddPeer(data []byte) ([]byte, error)
	HasPeer(data []byte) bool
	RemovePeer(data []byte) error
	Peers() ([]Peer, error)
	PeerCount() int

	// ParsePeerRequest turns the client's JSON peer request into the bytes
	// AddPeer expects; base64 of those bytes is the session key.
	ParsePeerRequest(raw []byte) ([]byte, error)
	// HandshakePayload renders the JSON-encodable configuration returned to
	// the client in result.data, from AddPeer's result.
	HandshakePayload(result []byte) (interface{}, error)
	// Metadata lists the service's inbounds for the node's root document and
	// the handshake; withPin includes per-node secrets such as the TLS pin,
	// which belong in the handshake but not in the public listing.
	Metadata(withPin bool) []Inbound
}

type Peer struct {
	Key      string `json:"key"`
	Upload   int64  `json:"upload"`
	Download int64  `json:"download"`
}

// Inbound describes one listener of the VPN service in the shape client apps
// read from a node's root document and handshake response. The numeric codes
// are the ones clients compare against.
type Inbound struct {
	Port              uint16 `json:"port"`
	ProxyProtocol     int    `json:"proxy_protocol"`
	TransportProtocol int    `json:"transport_protocol"`
	TransportSecurity int    `json:"transport_security"`
	TLSPin            string `json:"tls_pin,omitempty"`
}

const (
	ProxyProtocolVLESS = 1
	ProxyProtocolVMess = 2

	TransportProtocolTCP = 1

	TransportSecurityNone    = 1
	TransportSecurityTLS     = 2
	TransportSecurityReality = 3
)
