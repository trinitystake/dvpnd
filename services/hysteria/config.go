// SPDX-License-Identifier: Apache-2.0

package hysteria

import (
	"strings"
)

// configTemplate is the hysteria server configuration the node writes at
// start. One QUIC listener with the node's certificate; clients are
// authenticated by an HTTP call to the node; traffic is read from the
// statistics API. Strings are JSON-quoted, which YAML accepts.
var configTemplate = strings.TrimSpace(`
listen: ":{{ .Server.ListenPort }}"
tls:
  cert: {{ json .TLSCertPath }}
  key: {{ json .TLSKeyPath }}
{{- if .Server.ObfsPassword }}
obfs:
  type: salamander
  salamander:
    password: {{ json .Server.ObfsPassword }}
{{- end }}
auth:
  type: http
  http:
    url: "http://127.0.0.1:{{ .API.AuthPort }}/auth"
    insecure: false
trafficStats:
  listen: "127.0.0.1:{{ .API.StatsPort }}"
  secret: {{ json .StatsSecret }}
{{- if .Server.Up }}
bandwidth:
  up: {{ json .Server.Up }}
  down: {{ json .Server.Down }}
{{- end }}
ignoreClientBandwidth: false
disableUDP: false
`) + "\n"

// templateData is the configuration plus what only exists at runtime.
type templateData struct {
	Server interface{}
	API    interface{}

	TLSCertPath string
	TLSKeyPath  string
	StatsSecret string
}
