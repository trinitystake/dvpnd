// SPDX-License-Identifier: Apache-2.0

package types

import (
	"sort"
	"strings"
)

// nodeTypes is the set of protocol names the binary knows, filled by the
// services registry at init. Config validation checks [node] type against it;
// an empty set (a test binary without the registry) accepts any non-empty name.
var nodeTypes = map[string]nodeType{}

type nodeType struct {
	handshakeDNS bool
}

// RegisterNodeType declares a protocol name; handshakeDNS says whether the
// node may run the Handshake resolver next to it (a proxy cannot use it).
func RegisterNodeType(name string, handshakeDNS bool) {
	nodeTypes[name] = nodeType{handshakeDNS: handshakeDNS}
}

// NodeTypeNames lists the registered names, sorted.
func NodeTypeNames() []string {
	names := make([]string, 0, len(nodeTypes))
	for name := range nodeTypes {
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}

func nodeTypeKnown(name string) bool {
	if len(nodeTypes) == 0 {
		return true
	}
	_, ok := nodeTypes[name]

	return ok
}

func nodeTypeAllowsHandshakeDNS(name string) bool {
	t, ok := nodeTypes[name]
	if !ok {
		return true
	}

	return t.handshakeDNS
}

func nodeTypeList() string {
	return strings.Join(NodeTypeNames(), ", ")
}
