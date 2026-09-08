// SPDX-License-Identifier: Apache-2.0

package xray

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/trinitystake/dvpnd/services/common"
	xraytypes "github.com/trinitystake/dvpnd/services/xray/types"
	"github.com/trinitystake/dvpnd/types"
)

// Name is the service_type string clients expect.
const Name = "xray"

// NewService builds the XRAY service for a node.
func NewService(_ *types.Config) (types.Service, error) {
	return NewXRay(), nil
}

// Command is the "xray config …" CLI subtree.
func Command() *cobra.Command {
	return common.ConfigCommand(Name, nil, "XRAY sub-commands", common.ConfigSpec{
		FileName: xraytypes.ConfigFileName,
		Default:  func() common.ConfigFile { return xraytypes.NewConfig().WithDefaultValues() },
		Read:     func(v *viper.Viper) (common.ConfigFile, error) { return xraytypes.ReadInConfig(v) },
	})
}

func (s *XRay) Name() string {
	return Name
}

// ListenPort is the port encoded in the first two bytes of Info().
func (s *XRay) ListenPort() uint16 {
	return binary.BigEndian.Uint16(s.info[:2])
}

// ParsePeerRequest reads {"uuid": …} (a 16-byte array or the canonical
// string) and returns the VLESS byte followed by the 16 UUID bytes.
func (s *XRay) ParsePeerRequest(raw []byte) ([]byte, error) {
	var req struct {
		UUID json.RawMessage `json:"uuid"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("invalid xray peer request: %w", err)
	}

	id, err := common.UUIDFromJSON(req.UUID)
	if err != nil {
		return nil, err
	}

	return append([]byte{proxyVLESS}, id[:]...), nil
}

// HandshakePayloadData is what an XRAY client receives in result.data.
type HandshakePayloadData struct {
	Metadata []types.Inbound `json:"metadata"`
}

// HandshakePayload describes the VLESS inbound with everything the client
// needs to build its outbound: the certificate pin for TLS, or the REALITY
// public key, server name, short id and fingerprint.
func (s *XRay) HandshakePayload(_ []byte) (interface{}, error) {
	return HandshakePayloadData{Metadata: s.Metadata(true)}, nil
}

// Metadata lists the VLESS inbound. The public listing carries the codes
// only; the handshake (withPin) adds the pin or the REALITY parameters and
// the flow.
func (s *XRay) Metadata(withPin bool) []types.Inbound {
	entry := types.Inbound{
		Port:              s.ListenPort(),
		ProxyProtocol:     types.ProxyProtocolVLESS,
		TransportProtocol: types.TransportProtocolTCP,
		TransportSecurity: int(s.info[4]),
	}
	if !withPin {
		return []types.Inbound{entry}
	}

	if s.config.VLESS.Flow {
		entry.Flow = types.FlowVision
	}

	switch entry.TransportSecurity {
	case types.TransportSecurityTLS:
		entry.TLSPin = s.tlsPin
	case types.TransportSecurityReality:
		entry.RealityServerName = s.config.Reality.ServerName
		entry.RealityShortID = s.config.Reality.ShortID
		entry.RealityPublicKey = s.config.Reality.PublicKey
		entry.RealityFingerprint = s.config.Reality.Fingerprint
	}

	return []types.Inbound{entry}
}
