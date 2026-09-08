// SPDX-License-Identifier: Apache-2.0
// Modified from sentinel-official/dvpn-node @ 62bde16 (2024-01-25). See NOTICE.

package wireguard

import (
	"strings"
)

// The FORWARD rules accept traffic leaving the tunnel and the replies coming
// back into it. The second rule matters on a host whose FORWARD policy is DROP
// (ufw, or a host where Docker is or was running): without it every reply to a
// peer is dropped and TCP through the tunnel hangs.
//
// nolint:lll
var (
	configTemplate = strings.TrimSpace(`
[Interface]
Address = 10.8.0.1/24,fd86:ea04:1115::1/120
ListenPort = {{ .ListenPort }}
PrivateKey = {{ .PrivateKey }}
{{- range .Extra }}
{{ . }}
{{- end }}
PostUp = iptables -A FORWARD -i %i -j ACCEPT; iptables -A FORWARD -o %i -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT; iptables -t nat -A POSTROUTING -o {{ .Uplink }} -j MASQUERADE; ip6tables -A FORWARD -i %i -j ACCEPT; ip6tables -A FORWARD -o %i -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT; ip6tables -t nat -A POSTROUTING -o {{ .Uplink }} -j MASQUERADE;
PostDown = iptables -D FORWARD -i %i -j ACCEPT; iptables -D FORWARD -o %i -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT; iptables -t nat -D POSTROUTING -o {{ .Uplink }} -j MASQUERADE; ip6tables -D FORWARD -i %i -j ACCEPT; ip6tables -D FORWARD -o %i -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT; ip6tables -t nat -D POSTROUTING -o {{ .Uplink }} -j MASQUERADE;
    `)
)
