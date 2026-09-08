// SPDX-License-Identifier: Apache-2.0

package types

import (
	"strings"
	"testing"
)

func TestConfigDefaultsToDualStack(t *testing.T) {
	c := NewConfig().WithDefaultValues()
	if !c.EnableIPv6 {
		t.Fatal("IPv6 must be on by default, as on every upstream node")
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("default config must validate: %v", err)
	}
	out := c.String()
	if !strings.Contains(out, "enable_ipv6 = true") {
		t.Errorf("rendered config lacks enable_ipv6, got:\n%s", out)
	}
}
