// SPDX-License-Identifier: Apache-2.0

package hysteria

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/trinitystake/dvpnd/services/common"
	hysteriatypes "github.com/trinitystake/dvpnd/services/hysteria/types"
	"github.com/trinitystake/dvpnd/types"
)

// Name is the service_type string clients expect.
const Name = "hysteria2"

// NewService builds the Hysteria2 service for a node.
func NewService(_ *types.Config) (types.Service, error) {
	return NewHysteria(), nil
}

// Command is the "hysteria2 config …" CLI subtree.
func Command() *cobra.Command {
	return common.ConfigCommand(Name, []string{"hysteria", "hy2"}, "Hysteria2 sub-commands", common.ConfigSpec{
		FileName: hysteriatypes.ConfigFileName,
		Default:  func() common.ConfigFile { return hysteriatypes.NewConfig().WithDefaultValues() },
		Read:     func(v *viper.Viper) (common.ConfigFile, error) { return hysteriatypes.ReadInConfig(v) },
	})
}

func (s *Hysteria) Name() string {
	return Name
}

// ListenPort is the port encoded in the first two bytes of Info().
func (s *Hysteria) ListenPort() uint16 {
	return binary.BigEndian.Uint16(s.info[:2])
}

// ParsePeerRequest reads {"uuid": …}. Client apps send the canonical string
// for this node type (it doubles as the password they present); a 16-byte
// array is accepted too. The peer data is the 16 raw bytes.
func (s *Hysteria) ParsePeerRequest(raw []byte) ([]byte, error) {
	var req struct {
		UUID json.RawMessage `json:"uuid"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("invalid hysteria2 peer request: %w", err)
	}

	id, err := common.UUIDFromJSON(req.UUID)
	if err != nil {
		return nil, err
	}

	return id[:], nil
}

// HandshakePayloadData is what a Hysteria2 client receives in result.data.
type HandshakePayloadData struct {
	Metadata []types.Inbound `json:"metadata"`
}

// HandshakePayload gives the client the port, the certificate pin it must
// verify, and the obfuscation password when one is set.
func (s *Hysteria) HandshakePayload(_ []byte) (interface{}, error) {
	return HandshakePayloadData{Metadata: s.Metadata(true)}, nil
}

// Metadata lists the QUIC listener. There is no proxy or transport code for
// Hysteria2 in the client apps' vocabulary; security is TLS with a pin.
func (s *Hysteria) Metadata(withPin bool) []types.Inbound {
	entry := types.Inbound{
		Port:              s.ListenPort(),
		TransportSecurity: types.TransportSecurityTLS,
	}
	if withPin {
		entry.TLSPin = s.tlsPin
		entry.ObfsPassword = s.config.Server.ObfsPassword
	}

	return []types.Inbound{entry}
}
