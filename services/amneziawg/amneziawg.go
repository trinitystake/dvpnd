// SPDX-License-Identifier: Apache-2.0

// Package amneziawg runs an AmneziaWG tunnel as the node's VPN service.
// AmneziaWG is WireGuard with obfuscation: junk packets before the
// handshake, padding on the handshake messages and replaced message type
// values. The data plane, peers and usage are the WireGuard service's,
// driven with the awg tools; this package adds the parameters, which
// clients receive in the handshake. See docs/protocols.md.
package amneziawg

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	awgtypes "github.com/trinitystake/dvpnd/services/amneziawg/types"
	"github.com/trinitystake/dvpnd/services/common"
	"github.com/trinitystake/dvpnd/services/wireguard"
	wgtypes "github.com/trinitystake/dvpnd/services/wireguard/types"
	"github.com/trinitystake/dvpnd/types"
)

// Name is the service_type string clients expect.
const Name = "amneziawg"

// Variant drives the tunnel with the AmneziaWG tools.
var Variant = wireguard.Variant{
	Name:      Name,
	Type:      awgtypes.Type,
	Tool:      "awg",
	Quick:     "awg-quick",
	ConfigDir: "/etc/amnezia/amneziawg",
}

var (
	_ types.Service = (*AmneziaWG)(nil)

	// lookPath is how Init checks for the tools; a variable so tests can
	// stub it.
	lookPath = exec.LookPath
)

type AmneziaWG struct {
	*wireguard.WireGuard
	config *awgtypes.Config
}

// NewAmneziaWG builds the service over the given tunnel address pools.
func NewAmneziaWG(pool *wgtypes.IPPool) *AmneziaWG {
	return &AmneziaWG{
		WireGuard: wireguard.NewVariant(Variant, pool),
		config:    awgtypes.NewConfig(),
	}
}

// NewService builds the AmneziaWG service for a node.
func NewService(_ *types.Config) (types.Service, error) {
	ipv4Pool, err := wgtypes.NewIPv4PoolFromCIDR(types.IPv4CIDR)
	if err != nil {
		return nil, err
	}

	ipv6Pool, err := wgtypes.NewIPv6PoolFromCIDR(types.IPv6CIDR)
	if err != nil {
		return nil, err
	}

	return NewAmneziaWG(wgtypes.NewIPPool(ipv4Pool, ipv6Pool)), nil
}

// Command is the "amneziawg config …" CLI subtree.
func Command() *cobra.Command {
	return common.ConfigCommand(Name, []string{"awg"}, "AmneziaWG sub-commands", common.ConfigSpec{
		FileName: awgtypes.ConfigFileName,
		Default:  func() common.ConfigFile { return awgtypes.NewConfig().WithDefaultValues() },
		Read:     func(v *viper.Viper) (common.ConfigFile, error) { return awgtypes.ReadInConfig(v) },
	})
}

// Init reads amneziawg.toml, hands the WireGuard part and the obfuscation
// lines to the tunnel service, and lets it write the interface configuration.
func (s *AmneziaWG) Init(home string) (err error) {
	for _, tool := range []string{Variant.Tool, Variant.Quick} {
		if _, err = lookPath(tool); err != nil {
			return fmt.Errorf("the %q tool is not on PATH: install amneziawg-tools on the host "+
				"(docs/operator.md, section 6) or run the Docker image, which bundles it: %w", tool, err)
		}
	}

	v := viper.New()
	v.SetConfigFile(filepath.Join(home, awgtypes.ConfigFileName))

	s.config, err = awgtypes.ReadInConfig(v)
	if err != nil {
		return err
	}
	if err = s.config.Validate(); err != nil {
		return err
	}

	s.WireGuard.WithConfig(s.config.WireGuard(), s.config.Obfuscation.InterfaceLines())

	return s.WireGuard.Init(home)
}

// Obfuscation is the parameter set in force after Init.
func (s *AmneziaWG) Obfuscation() *awgtypes.Obfuscation {
	return s.config.Obfuscation
}

// PublicInbound is the AmneziaWG entry of the root document: everything a
// client needs comes with the handshake, so all of it is blank in public.
type PublicInbound struct {
	Port      int     `json:"port"`
	PublicKey *string `json:"public_key"`
	S1        uint16  `json:"s1"`
	S2        uint16  `json:"s2"`
	S3        uint16  `json:"s3"`
	S4        uint16  `json:"s4"`
	H1        uint32  `json:"h1"`
	H2        uint32  `json:"h2"`
	H3        uint32  `json:"h3"`
	H4        uint32  `json:"h4"`
}

// PublicMetadata lists the single listener with its details blanked.
func (s *AmneziaWG) PublicMetadata() interface{} {
	return []PublicInbound{{}}
}

// PeerMetadata is WireGuard's endpoint details plus the parameters the
// client must match. Junk packet counts are per side and not sent.
type PeerMetadata struct {
	wireguard.PeerMetadata
	S1 uint16 `json:"s1"`
	S2 uint16 `json:"s2"`
	S3 uint16 `json:"s3"`
	S4 uint16 `json:"s4"`
	H1 uint32 `json:"h1"`
	H2 uint32 `json:"h2"`
	H3 uint32 `json:"h3"`
	H4 uint32 `json:"h4"`
	I1 string `json:"i1,omitempty"`
	I2 string `json:"i2,omitempty"`
	I3 string `json:"i3,omitempty"`
	I4 string `json:"i4,omitempty"`
	I5 string `json:"i5,omitempty"`
}

// HandshakePayloadData is what an AmneziaWG client receives in result.data.
type HandshakePayloadData struct {
	Addrs    []string       `json:"addrs"`
	Metadata []PeerMetadata `json:"metadata"`
}

// HandshakePayload is WireGuard's payload with the obfuscation parameters
// added to the metadata entry.
func (s *AmneziaWG) HandshakePayload(result []byte) (interface{}, error) {
	base, err := s.WireGuard.HandshakePayload(result)
	if err != nil {
		return nil, err
	}
	wg := base.(wireguard.HandshakePayloadData)

	o := s.config.Obfuscation
	entry := PeerMetadata{
		PeerMetadata: wg.Metadata[0],
		S1:           o.S1, S2: o.S2, S3: o.S3, S4: o.S4,
		H1: o.H1, H2: o.H2, H3: o.H3, H4: o.H4,
		I1: o.I1, I2: o.I2, I3: o.I3, I4: o.I4, I5: o.I5,
	}

	return HandshakePayloadData{Addrs: wg.Addrs, Metadata: []PeerMetadata{entry}}, nil
}

// String renders the payload for logs and tests.
func (p HandshakePayloadData) String() string {
	b, _ := json.Marshal(p)

	return string(b)
}
