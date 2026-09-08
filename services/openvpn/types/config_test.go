// SPDX-License-Identifier: Apache-2.0

package types

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestConfig(t *testing.T) {
	c := NewConfig().WithDefaultValues()
	if err := c.Validate(); err != nil {
		t.Fatalf("defaults: %v", err)
	}
	path := filepath.Join(t.TempDir(), ConfigFileName)
	c.Proto = ProtoTCP
	if err := c.SaveToPath(path); err != nil {
		t.Fatal(err)
	}
	v := viper.New()
	v.SetConfigFile(path)
	read, err := ReadInConfig(v)
	if err != nil {
		t.Fatal(err)
	}
	if read.String() != c.String() || read.Proto != ProtoTCP || read.Interface != "ovpn0" {
		t.Fatalf("round trip:\n%s", read.String())
	}

	cases := []struct {
		name string
		edit func(*Config)
		want string
	}{
		{"proto", func(c *Config) { c.Proto = "sctp" }, "proto must be"},
		{"port", func(c *Config) { c.ListenPort = 0 }, "listen_port"},
		{"management", func(c *Config) { c.Management.Port = 0 }, "management port"},
		{"same ports", func(c *Config) { c.Management.Port = c.ListenPort }, "must differ"},
		{"interface", func(c *Config) { c.Interface = "a b" }, "interface"},
	}
	for _, tc := range cases {
		c := NewConfig().WithDefaultValues()
		tc.edit(c)
		err := c.Validate()
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: got %v, want %q", tc.name, err, tc.want)
		}
	}
}
