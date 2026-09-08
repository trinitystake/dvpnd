// SPDX-License-Identifier: Apache-2.0

package types

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestConfigDefaultsAndValidation(t *testing.T) {
	c := NewConfig().WithDefaultValues()
	if err := c.Validate(); err != nil {
		t.Fatalf("defaults must validate: %v", err)
	}

	path := filepath.Join(t.TempDir(), ConfigFileName)
	c.Server.ObfsPassword = "s3cret"
	c.Server.Up, c.Server.Down = "100 mbps", "1 gbps"
	if err := c.SaveToPath(path); err != nil {
		t.Fatal(err)
	}
	v := viper.New()
	v.SetConfigFile(path)
	read, err := ReadInConfig(v)
	if err != nil {
		t.Fatal(err)
	}
	if read.String() != c.String() || read.Server.ObfsPassword != "s3cret" {
		t.Fatalf("round trip differs:\n%s", read.String())
	}

	cases := []struct {
		name string
		edit func(*Config)
		want string
	}{
		{"zero port", func(c *Config) { c.Server.ListenPort = 0 }, "listen_port"},
		{"bad rate", func(c *Config) { c.Server.Up, c.Server.Down = "fast", "1 gbps" }, "up must be a rate"},
		{"one of two", func(c *Config) { c.Server.Up = "" }, "set together"},
		{"quote in password", func(c *Config) { c.Server.ObfsPassword = `a"b` }, "obfs_password"},
		{"same api ports", func(c *Config) { c.API.StatsPort = c.API.AuthPort }, "must differ"},
		{"zero api port", func(c *Config) { c.API.AuthPort = 0 }, "cannot be zero"},
	}
	for _, tc := range cases {
		c := NewConfig().WithDefaultValues()
		c.Server.Up, c.Server.Down = "100 mbps", "1 gbps"
		tc.edit(c)
		err := c.Validate()
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: got %v, want error containing %q", tc.name, err, tc.want)
		}
	}
}
