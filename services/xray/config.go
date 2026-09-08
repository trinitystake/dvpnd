// SPDX-License-Identifier: Apache-2.0

package xray

import (
	"strings"
)

// configTemplate is the xray server configuration the node writes at start.
// One VLESS inbound over raw TCP, wrapped in TLS with the node's certificate
// or in REALITY; peers are added and removed over the gRPC API on the
// loopback inbound, and per-user traffic statistics are switched on.
// Private and loopback destinations are blocked explicitly (not with
// geoip:private, which needs the geoip asset file) so a client cannot reach
// the host's own services, the control API first of all.
var configTemplate = strings.TrimSpace(`
{
    "log": {
        "access": "none",
        "loglevel": "warning"
    },
    "api": {
        "tag": "api",
        "services": [
            "HandlerService",
            "StatsService"
        ]
    },
    "stats": {},
    "policy": {
        "levels": {
            "0": {
                "statsUserUplink": true,
                "statsUserDownlink": true
            }
        }
    },
    "inbounds": [
        {
            "tag": "api",
            "listen": "127.0.0.1",
            "port": {{ .API.Port }},
            "protocol": "dokodemo-door",
            "settings": {
                "address": "127.0.0.1"
            }
        },
        {
            "tag": "vless",
            "port": {{ .VLESS.ListenPort }},
            "protocol": "vless",
            "settings": {
                "clients": [],
                "decryption": "none"
            },
            "streamSettings": {
                "network": "tcp",
                "security": "{{ .VLESS.Security }}",
{{- if eq .VLESS.Security "tls" }}
                "tlsSettings": {
                    "minVersion": "1.2",
                    "certificates": [
                        {
                            "certificateFile": "{{ .TLSCertPath }}",
                            "keyFile": "{{ .TLSKeyPath }}"
                        }
                    ]
                }
{{- else }}
                "realitySettings": {
                    "show": false,
                    "dest": "{{ .Reality.Dest }}",
                    "xver": 0,
                    "serverNames": [
                        "{{ .Reality.ServerName }}"
                    ],
                    "privateKey": "{{ .Reality.PrivateKey }}",
                    "shortIds": [
                        "{{ .Reality.ShortID }}"
                    ]
                }
{{- end }}
            }
        }
    ],
    "outbounds": [
        {
            "tag": "direct",
            "protocol": "freedom"
        },
        {
            "tag": "blocked",
            "protocol": "blackhole"
        }
    ],
    "routing": {
        "rules": [
            {
                "type": "field",
                "inboundTag": [
                    "api"
                ],
                "outboundTag": "api"
            },
            {
                "type": "field",
                "ip": [
                    "0.0.0.0/8",
                    "10.0.0.0/8",
                    "100.64.0.0/10",
                    "127.0.0.0/8",
                    "169.254.0.0/16",
                    "172.16.0.0/12",
                    "192.168.0.0/16",
                    "::1/128",
                    "fc00::/7",
                    "fe80::/10"
                ],
                "outboundTag": "blocked"
            }
        ]
    }
}
`)

// templateData is the configuration plus the TLS file paths, which are not
// in xray.toml because they are always the node's own.
type templateData struct {
	VLESS   interface{}
	Reality interface{}
	API     interface{}

	TLSCertPath string
	TLSKeyPath  string
}
