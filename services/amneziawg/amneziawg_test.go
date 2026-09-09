// SPDX-License-Identifier: Apache-2.0

package amneziawg

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	awgtypes "github.com/trinitystake/dvpnd/v9/services/amneziawg/types"
	wgtypes "github.com/trinitystake/dvpnd/v9/services/wireguard/types"
)

func setup(t *testing.T) (string, *awgtypes.Config, *AmneziaWG) {
	t.Helper()

	saved := lookPath
	lookPath = func(string) (string, error) { return "/usr/bin/awg", nil }
	t.Cleanup(func() { lookPath = saved })

	confDir := t.TempDir()
	savedDir := Variant.ConfigDir
	Variant.ConfigDir = confDir
	t.Cleanup(func() { Variant.ConfigDir = savedDir })

	home := t.TempDir()
	cfg := awgtypes.NewConfig().WithDefaultValues()
	cfg.ListenPort = 51821
	cfg.Uplink = "eth0"
	cfg.Obfuscation.I1 = "<b 0xabcd><r 8>"
	if err := cfg.SaveToPath(filepath.Join(home, awgtypes.ConfigFileName)); err != nil {
		t.Fatal(err)
	}

	v4, _ := wgtypes.NewIPv4PoolFromCIDR("10.8.0.2/24")
	v6, _ := wgtypes.NewIPv6PoolFromCIDR("fd86:ea04:1115::2/120")

	return confDir, cfg, NewAmneziaWG(wgtypes.NewIPPool(v4, v6))
}

func TestInitWritesInterfaceConfig(t *testing.T) {
	confDir, cfg, s := setup(t)

	if err := s.Init(filepath.Dir(filepath.Join(t.TempDir(), "x"))); err == nil {
		t.Fatal("Init without amneziawg.toml must fail")
	}

	home := filepath.Dir(cfgPath(t, cfg))
	if err := s.Init(home); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(confDir, "awg0.conf"))
	if err != nil {
		t.Fatal(err)
	}
	out := string(raw)
	o := cfg.Obfuscation
	for _, want := range []string{
		"ListenPort = 51821",
		"PrivateKey = " + cfg.PrivateKey,
		"Jc = 4", "Jmin = 40", "Jmax = 70",
		"S1 = " + itoa(uint64(o.S1)), "S2 = " + itoa(uint64(o.S2)), "S3 = 0", "S4 = 0",
		"H1 = " + itoa(uint64(o.H1)), "H4 = " + itoa(uint64(o.H4)),
		"I1 = <b 0xabcd><r 8>",
		"-o eth0 -j MASQUERADE",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("awg0.conf lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "I2 =") {
		t.Error("empty signature packets must not be written")
	}
	if s.Name() != "amneziawg" || s.Type() != 5 || s.ListenPort() != 51821 {
		t.Fatalf("identity: %s/%d port %d", s.Name(), s.Type(), s.ListenPort())
	}
	if s.Config().Interface != "awg0" {
		t.Fatalf("interface: %s", s.Config().Interface)
	}
}

func TestHandshakePayload(t *testing.T) {
	_, cfg, s := setup(t)
	if err := s.Init(filepath.Dir(cfgPath(t, cfg))); err != nil {
		t.Fatal(err)
	}

	v4 := []byte{10, 8, 0, 3}
	v6 := []byte{0xfd, 0x86, 0xea, 0x04, 0x11, 0x15, 0, 0, 0, 0, 0, 0, 0, 0, 0, 3}
	payload, err := s.HandshakePayload(append(append([]byte{}, v4...), v6...))
	if err != nil {
		t.Fatal(err)
	}
	out, _ := json.Marshal(payload)

	o := cfg.Obfuscation
	want := `{"addrs":["10.8.0.3/32","fd86:ea04:1115::3/128"],"metadata":[{"port":51821,"public_key":"` + s.PublicKey() + `",` +
		`"s1":` + itoa(uint64(o.S1)) + `,"s2":` + itoa(uint64(o.S2)) + `,"s3":0,"s4":0,` +
		`"h1":` + itoa(uint64(o.H1)) + `,"h2":` + itoa(uint64(o.H2)) + `,"h3":` + itoa(uint64(o.H3)) + `,"h4":` + itoa(uint64(o.H4)) + `,` +
		`"i1":"<b 0xabcd><r 8>"}]}`
	// encoding/json escapes < and > as \u003c and \u003e; compare decoded.
	var got, exp interface{}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want), &exp); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, exp) {
		t.Fatalf("payload:\n got %s\nwant %s", out, want)
	}

	if pub, _ := json.Marshal(s.PublicMetadata()); string(pub) != `[{"port":0,"public_key":null,"s1":0,"s2":0,"s3":0,"s4":0,"h1":0,"h2":0,"h3":0,"h4":0}]` {
		t.Fatalf("public metadata: %s", pub)
	}
	if _, err := s.HandshakePayload([]byte{1}); err == nil {
		t.Fatal("short result accepted")
	}
}

func TestInitRequiresTools(t *testing.T) {
	_, cfg, s := setup(t)
	lookPath = func(name string) (string, error) { return "", errors.New("not found: " + name) }

	err := s.Init(filepath.Dir(cfgPath(t, cfg)))
	if err == nil || !strings.Contains(err.Error(), `"awg" tool is not on PATH`) {
		t.Fatalf("Init without the tools: %v", err)
	}
}

// cfgPath finds the config file setup wrote (the home is its directory).
func cfgPath(t *testing.T, cfg *awgtypes.Config) string {
	t.Helper()
	// setup saved the file under a temp home; write it again under a known one.
	home := t.TempDir()
	path := filepath.Join(home, awgtypes.ConfigFileName)
	if err := cfg.SaveToPath(path); err != nil {
		t.Fatal(err)
	}
	return path
}

func itoa(v uint64) string {
	b, _ := json.Marshal(v)
	return string(b)
}
