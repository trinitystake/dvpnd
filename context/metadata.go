// SPDX-License-Identifier: Apache-2.0

package context

import (
	"encoding/base64"
	"encoding/binary"

	v2raytypes "github.com/trinitystake/dvpnd/services/v2ray/types"
	wgtypes "github.com/trinitystake/dvpnd/services/wireguard/types"
)

// Inbound describes one listener of the VPN service in the shape current
// clients read from a node's root document and handshake response. The numeric
// codes are the ones clients compare against: proxy_protocol 1 = VLESS,
// 2 = VMess; transport_security 1 = none, 2 = TLS, 3 = Reality;
// transport_protocol 1 = TCP (the only value confirmed with current clients).
type Inbound struct {
	Port              uint16 `json:"port"`
	ProxyProtocol     int    `json:"proxy_protocol"`
	TransportProtocol int    `json:"transport_protocol"`
	TransportSecurity int    `json:"transport_security"`
	TLSPin            string `json:"tls_pin,omitempty"`
}

const (
	proxyVMess        = 2
	transportSecNone  = 1
	transportSecTLS   = 2
	transportProtoTCP = 1
)

// ServiceTypeName is the string form clients expect in service_type.
func (c *Context) ServiceTypeName() string {
	switch c.Service().Type() {
	case wgtypes.Type:
		return "wireguard"
	case v2raytypes.Type:
		return "v2ray"
	default:
		return ""
	}
}

// ServicePort is the listen port encoded in the first two bytes of Service().Info().
func (c *Context) ServicePort() uint16 {
	return binary.BigEndian.Uint16(c.Service().Info()[:2])
}

// WireGuardPublicKey is the interface public key from Service().Info(), base64.
func (c *Context) WireGuardPublicKey() string {
	return base64.StdEncoding.EncodeToString(c.Service().Info()[2:])
}

// TLSPin returns the SHA-256 (hex) of the TLS certificate the service presents,
// or "" when the service has no TLS or does not expose one.
func (c *Context) TLSPin() string {
	if s, ok := c.Service().(interface{ TLSPin() string }); ok {
		return s.TLSPin()
	}

	return ""
}

// ServiceMetadata lists the service's inbounds. withPin includes the per-node
// TLS pin, which belongs in the handshake response but not in the public
// listing.
func (c *Context) ServiceMetadata(withPin bool) []Inbound {
	info := c.Service().Info()
	entry := Inbound{Port: c.ServicePort()}

	if c.Service().Type() == v2raytypes.Type {
		entry.ProxyProtocol = proxyVMess
		entry.TransportProtocol = transportProtocolCode(info[2])
		entry.TransportSecurity = transportSecNone
		if info[3] != 0 {
			entry.TransportSecurity = transportSecTLS
			if withPin {
				entry.TLSPin = c.TLSPin()
			}
		}
	}

	return []Inbound{entry}
}

// transportProtocolCode maps this node's V2Ray transport byte to the code
// clients use. Only TCP is confirmed against current clients; the other
// transports keep their historical byte value, which may or may not match.
func transportProtocolCode(b byte) int {
	if v2raytypes.Transport(b).String() == "tcp" {
		return transportProtoTCP
	}

	return int(b)
}
