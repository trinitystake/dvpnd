// SPDX-License-Identifier: Apache-2.0

// Package services is the registry of VPN protocols the node can run. Adding a
// protocol means implementing types.Service in a package under services/ and
// adding one entry to the list below; the node's CLI, configuration
// validation, status document and handshake all go through the registry.
package services

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/trinitystake/dvpnd/services/v2ray"
	v2raytypes "github.com/trinitystake/dvpnd/services/v2ray/types"
	"github.com/trinitystake/dvpnd/services/wireguard"
	wgtypes "github.com/trinitystake/dvpnd/services/wireguard/types"
	"github.com/trinitystake/dvpnd/services/xray"
	xraytypes "github.com/trinitystake/dvpnd/services/xray/types"
	"github.com/trinitystake/dvpnd/types"
)

// Protocol is one registered VPN protocol.
type Protocol struct {
	// Name is the [node] type value and the service_type clients see.
	Name string
	// Type is the numeric service type clients see in /status.
	Type uint64
	// New builds the service for a node with this configuration.
	New func(cfg *types.Config) (types.Service, error)
	// Command is the protocol's CLI subtree ("<name> config init|show|set").
	Command func() *cobra.Command
	// HandshakeDNS says whether the node may run the Handshake resolver next
	// to this service; a proxy cannot make use of it.
	HandshakeDNS bool
}

var registry = []Protocol{
	{Name: wireguard.Name, Type: wgtypes.Type, New: wireguard.NewService, Command: wireguard.Command, HandshakeDNS: true},
	{Name: v2ray.Name, Type: v2raytypes.Type, New: v2ray.NewService, Command: v2ray.Command},
	{Name: xray.Name, Type: xraytypes.Type, New: xray.NewService, Command: xray.Command},
}

func init() {
	for _, p := range registry {
		types.RegisterNodeType(p.Name, p.HandshakeDNS)
	}
}

// All lists the registered protocols in registration order.
func All() []Protocol {
	return append([]Protocol(nil), registry...)
}

// Lookup finds a protocol by its [node] type name.
func Lookup(name string) (Protocol, error) {
	for _, p := range registry {
		if p.Name == name {
			return p, nil
		}
	}

	return Protocol{}, fmt.Errorf("unknown node type %q; one of: %s", name, strings.Join(Names(), ", "))
}

// Names lists the registered protocol names in registration order.
func Names() []string {
	names := make([]string, 0, len(registry))
	for _, p := range registry {
		names = append(names, p.Name)
	}

	return names
}
