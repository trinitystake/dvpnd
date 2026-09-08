// SPDX-License-Identifier: Apache-2.0

package openvpn

import (
	"strings"
)

// configTemplate is the server configuration the node writes at start. TLS
// settings match what the client apps' profile pins: ECDSA certificates,
// AES-GCM data channel, SHA256 HMAC, tls-crypt. Clients are admitted over the
// management interface (management-client-auth), not by a script; their
// credential is the certificate, so no username or password is demanded.
var configTemplate = strings.TrimSpace(`
dev {{ .Interface }}
dev-type tun
proto {{ if eq .Proto "tcp" }}tcp-server{{ else }}udp{{ end }}
port {{ .ListenPort }}
server {{ .IPv4Network }} {{ .IPv4Netmask }}
{{- if .EnableIPv6 }}
server-ipv6 {{ .IPv6Network }}
{{- end }}
topology subnet
ca {{ .CACert }}
cert {{ .ServerCert }}
key {{ .ServerKey }}
dh none
tls-crypt {{ .TLSCrypt }}
tls-server
tls-version-min 1.2
auth SHA256
data-ciphers AES-256-GCM:AES-128-GCM
data-ciphers-fallback AES-256-GCM
tls-cipher TLS-ECDHE-ECDSA-WITH-AES-256-GCM-SHA384
remote-cert-tls client
keepalive 10 60
persist-key
persist-tun
{{- if eq .Proto "udp" }}
explicit-exit-notify 1
{{- end }}
management 127.0.0.1 {{ .ManagementPort }}
management-client-auth
auth-user-pass-optional
verb 3
`) + "\n"

// templateData is what the server configuration renders from.
type templateData struct {
	Interface      string
	Proto          string
	ListenPort     uint16
	EnableIPv6     bool
	IPv4Network    string
	IPv4Netmask    string
	IPv6Network    string
	CACert         string
	ServerCert     string
	ServerKey      string
	TLSCrypt       string
	ManagementPort uint16
}
