// SPDX-License-Identifier: Apache-2.0

package types

const (
	// Type is the numeric service type clients use for XRAY nodes.
	Type = 4

	ConfigFileName = "xray.toml"

	SecurityTLS     = "tls"
	SecurityReality = "reality"

	// InboundTag is the tag of the VLESS inbound in the generated xray config.
	InboundTag = "vless"
)
