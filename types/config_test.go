// SPDX-License-Identifier: Apache-2.0

package types

import (
	"strings"
	"testing"

	"github.com/trinitystake/dvpnd/libs/geoip"
)

func TestGeoIPConfigValidate(t *testing.T) {
	if c := NewGeoIPConfig().WithDefaultValues(); c.Provider != geoip.ProviderAuto {
		t.Fatalf("default provider is %q, want auto", c.Provider)
	} else if err := c.Validate(); err != nil {
		t.Fatalf("default config must validate: %v", err)
	}

	cases := []struct {
		name string
		cfg  GeoIPConfig
		want string // substring of the error, empty when the config must be valid
	}{
		{"auto", GeoIPConfig{Provider: "auto"}, ""},
		{"ipwhois", GeoIPConfig{Provider: "ipwhois"}, ""},
		{"ip2location", GeoIPConfig{Provider: "ip2location"}, ""},
		{"ip2location with key", GeoIPConfig{Provider: "ip2location", Token: "k"}, ""},
		{"cloudflare", GeoIPConfig{Provider: "cloudflare"}, ""},
		{"ipify", GeoIPConfig{Provider: "ipify"}, ""},
		{"ip-api with paid url", GeoIPConfig{Provider: "ip-api", URL: "https://pro.ip-api.com/json/?key=k"}, ""},
		{"ipinfo with token", GeoIPConfig{Provider: "ipinfo", Token: "t"}, ""},
		{"none", GeoIPConfig{Provider: "none"}, ""},
		{"static values", GeoIPConfig{Provider: "auto", City: "Turin", Country: "IT", Latitude: 45.07, Longitude: 7.69}, ""},

		{"unknown", GeoIPConfig{Provider: "bogus"}, "provider must be one of"},
		{"empty", GeoIPConfig{}, "provider must be one of"},
		{"auto with url", GeoIPConfig{Provider: "auto", URL: "https://example.com"}, "url cannot be set"},
		{"none with url", GeoIPConfig{Provider: "none", URL: "https://example.com"}, "url cannot be set"},
		{"auto with token", GeoIPConfig{Provider: "auto", Token: "t"}, "token is only used"},
		{"ipify with token", GeoIPConfig{Provider: "ipify", Token: "t"}, "token is only used"},
		{"ipinfo without token", GeoIPConfig{Provider: "ipinfo"}, "token is required"},
		{"bad url", GeoIPConfig{Provider: "ipwhois", URL: "not a url"}, "invalid url"},
		{"latitude", GeoIPConfig{Provider: "auto", Latitude: 91}, "latitude"},
		{"longitude", GeoIPConfig{Provider: "auto", Longitude: -181}, "longitude"},
	}
	for _, c := range cases {
		err := c.cfg.Validate()
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%s: unexpected error %v", c.name, err)
		case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
			t.Errorf("%s: got %v, want error containing %q", c.name, err, c.want)
		}
	}
}

func TestConfigTemplateRenders(t *testing.T) {
	out := NewConfig().WithDefaultValues().String()
	for _, want := range []string{`provider = "auto"`, "[geoip]", `city = ""`, "latitude = 0.000000"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered config lacks %q", want)
		}
	}
}
