// SPDX-License-Identifier: Apache-2.0

package wireguard

import (
	"bytes"
	"strings"
	"testing"
	"text/template"

	wgtypes "github.com/trinitystake/dvpnd/services/wireguard/types"
)

func renderConfig(t *testing.T, cfg *wgtypes.Config) string {
	t.Helper()

	tmpl, err := template.New("wireguard_conf").Parse(configTemplate)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cfg); err != nil {
		t.Fatal(err)
	}

	return buf.String()
}

func TestConfigTemplateForwardRules(t *testing.T) {
	cfg := wgtypes.NewConfig().WithDefaultValues()
	cfg.ListenPort = 51820
	cfg.PrivateKey = "cHJpdmF0ZQ=="
	cfg.Uplink = "eth0"

	out := renderConfig(t, cfg)

	lines := map[string]string{}
	for _, l := range strings.Split(out, "\n") {
		if k, v, ok := strings.Cut(l, " = "); ok {
			lines[k] = v
		}
	}

	if lines["ListenPort"] != "51820" {
		t.Fatalf("ListenPort = %q", lines["ListenPort"])
	}

	for _, tool := range []string{"iptables", "ip6tables"} {
		want := []string{
			tool + " -A FORWARD -i %i -j ACCEPT;",
			tool + " -A FORWARD -o %i -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT;",
			tool + " -t nat -A POSTROUTING -o eth0 -j MASQUERADE;",
		}
		for _, rule := range want {
			if !strings.Contains(lines["PostUp"], rule) {
				t.Errorf("PostUp lacks %q", rule)
			}
			down := strings.Replace(rule, " -A ", " -D ", 1)
			if !strings.Contains(lines["PostDown"], down) {
				t.Errorf("PostDown lacks %q", down)
			}
		}
	}

	// The accept rules come before the NAT rule so a reply is accepted
	// whatever the chain policy is.
	up := lines["PostUp"]
	if strings.Index(up, "-o %i -m conntrack") > strings.Index(up, "MASQUERADE") {
		t.Errorf("return-path rule must precede MASQUERADE: %s", up)
	}
}
