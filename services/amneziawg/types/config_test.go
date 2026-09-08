// SPDX-License-Identifier: Apache-2.0

package types

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestDefaultsValidateAndRoundTrip(t *testing.T) {
	c := NewConfig().WithDefaultValues()
	if err := c.Validate(); err != nil {
		t.Fatalf("defaults must validate: %v", err)
	}
	o := c.Obfuscation
	if o.S1 < 15 || o.S1 > 150 || o.S2 < 15 || o.S2 > 150 || o.S3 != 0 || o.S4 != 0 {
		t.Fatalf("paddings: %+v", o)
	}
	if o.H1 <= 4 || o.H1 == o.H2 || o.H2 == o.H3 || o.H3 == o.H4 {
		t.Fatalf("headers: %+v", o)
	}

	path := filepath.Join(t.TempDir(), ConfigFileName)
	c.Obfuscation.I1 = "<b 0x1234><r 16>"
	if err := c.SaveToPath(path); err != nil {
		t.Fatal(err)
	}
	v := viper.New()
	v.SetConfigFile(path)
	read, err := ReadInConfig(v)
	if err != nil {
		t.Fatal(err)
	}
	if read.String() != c.String() {
		t.Fatalf("round trip differs:\n%s\n---\n%s", read.String(), c.String())
	}
	if err := read.Validate(); err != nil {
		t.Fatal(err)
	}

	lines := read.Obfuscation.InterfaceLines()
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"Jc = 4", "Jmin = 40", "Jmax = 70", "S3 = 0", "I1 = <b 0x1234><r 16>"} {
		if !strings.Contains(joined, want) {
			t.Errorf("interface lines lack %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "I2 =") {
		t.Error("empty signature packets must be omitted")
	}
	if wg := read.WireGuard(); wg.Interface != "awg0" || wg.ListenPort != c.ListenPort || wg.PrivateKey != c.PrivateKey {
		t.Fatalf("wireguard part: %+v", wg)
	}
}

func TestObfuscationValidation(t *testing.T) {
	cases := []struct {
		name string
		edit func(*Obfuscation)
		want string
	}{
		{"jc too big", func(o *Obfuscation) { o.Jc = 129 }, "jc"},
		{"jmin above jmax", func(o *Obfuscation) { o.Jmin, o.Jmax = 80, 70 }, "jmin"},
		{"s2 collides", func(o *Obfuscation) { o.S1, o.S2 = 20, 76 }, "s1 + 56"},
		{"s1 too big", func(o *Obfuscation) { o.S1 = 2000 }, "at most 1132"},
		{"header too small", func(o *Obfuscation) { o.H1 = 3 }, "above 4"},
		{"headers not distinct", func(o *Obfuscation) { o.H2 = o.H1 }, "distinct"},
		{"bad i packet", func(o *Obfuscation) { o.I3 = "junk" }, "i3"},
	}
	for _, tc := range cases {
		o := (&Obfuscation{}).Generate()
		tc.edit(o)
		err := o.Validate()
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: got %v, want error containing %q", tc.name, err, tc.want)
		}
	}

	// All-zero headers are plain WireGuard framing and allowed.
	o := (&Obfuscation{}).Generate()
	o.H1, o.H2, o.H3, o.H4 = 0, 0, 0, 0
	if err := o.Validate(); err != nil {
		t.Fatalf("all-zero headers: %v", err)
	}
}
