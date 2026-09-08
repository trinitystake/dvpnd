// SPDX-License-Identifier: Apache-2.0

package v2ray

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/trinitystake/dvpnd/services/common"
	v2raytypes "github.com/trinitystake/dvpnd/services/v2ray/types"
	"github.com/trinitystake/dvpnd/types"
)

// Name is the service_type string clients expect.
const Name = "v2ray"

// NewService builds the V2Ray service for a node.
func NewService(_ *types.Config) (types.Service, error) {
	return NewV2Ray(), nil
}

// Command is the "v2ray config …" CLI subtree.
func Command() *cobra.Command {
	return common.ConfigCommand(Name, nil, "V2Ray sub-commands", common.ConfigSpec{
		FileName: v2raytypes.ConfigFileName,
		Default:  func() common.ConfigFile { return v2raytypes.NewConfig().WithDefaultValues() },
		Read:     func(v *viper.Viper) (common.ConfigFile, error) { return v2raytypes.ReadInConfig(v) },
	})
}

func (s *V2Ray) Name() string {
	return Name
}

// ListenPort is the port encoded in the first two bytes of Info().
func (s *V2Ray) ListenPort() uint16 {
	return binary.BigEndian.Uint16(s.info[:2])
}

// ParsePeerRequest reads {"uuid": …} (a 16-byte array or the canonical
// string) and returns the VMess proxy byte followed by the 16 UUID bytes,
// which is what AddPeer expects.
func (s *V2Ray) ParsePeerRequest(raw []byte) ([]byte, error) {
	var req struct {
		UUID json.RawMessage `json:"uuid"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("invalid v2ray peer request: %w", err)
	}

	id, err := common.UUIDFromJSON(req.UUID)
	if err != nil {
		return nil, err
	}

	return append([]byte{v2raytypes.Proxy(0x01).Byte()}, id[:]...), nil
}

// HandshakePayloadData is what a V2Ray client receives in result.data.
type HandshakePayloadData struct {
	Metadata []types.Inbound `json:"metadata"`
}

// HandshakePayload describes the VMess inbound, with the TLS pin when the
// inbound is wrapped in TLS. AddPeer returns nothing V2Ray clients need.
func (s *V2Ray) HandshakePayload(_ []byte) (interface{}, error) {
	return HandshakePayloadData{Metadata: s.Metadata(true)}, nil
}

// Metadata lists the VMess inbound: transport code, and TLS with the pin when
// withPin is set.
func (s *V2Ray) Metadata(withPin bool) []types.Inbound {
	entry := types.Inbound{
		Port:              s.ListenPort(),
		ProxyProtocol:     types.ProxyProtocolVMess,
		TransportProtocol: transportProtocolCode(s.info[2]),
		TransportSecurity: types.TransportSecurityNone,
	}
	if s.info[3] != 0 {
		entry.TransportSecurity = types.TransportSecurityTLS
		if withPin {
			entry.TLSPin = s.tlsPin
		}
	}

	return []types.Inbound{entry}
}

// transportProtocolCode maps this node's transport byte to the code clients
// use. Only TCP is confirmed against current clients; the other transports
// keep their historical byte value, which may or may not match.
func transportProtocolCode(b byte) int {
	if v2raytypes.Transport(b).String() == "tcp" {
		return types.TransportProtocolTCP
	}

	return int(b)
}
