// SPDX-License-Identifier: Apache-2.0

package geoip

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		provider, body    string
		ip, city, country string
		lat, lon          float64
	}{
		{ProviderIPify, `{"ip":"203.0.113.7"}`, "203.0.113.7", "", "", 0, 0},
		{ProviderIPAPI, `{"status":"success","country":"Italy","city":"Milan","lat":45.46,"lon":9.19,"query":"203.0.113.7"}`,
			"203.0.113.7", "Milan", "Italy", 45.46, 9.19},
		{ProviderIPInfo, `{"ip":"203.0.113.7","city":"Milan","country":"IT","loc":"45.4600,9.1900"}`,
			"203.0.113.7", "Milan", "IT", 45.46, 9.19},
	}
	for _, c := range cases {
		loc, err := parse(c.provider, json.NewDecoder(strings.NewReader(c.body)))
		if err != nil {
			t.Fatalf("%s: %v", c.provider, err)
		}
		if loc.IP != c.ip || loc.City != c.city || loc.Country != c.country || loc.Latitude != c.lat || loc.Longitude != c.lon {
			t.Errorf("%s: got %+v", c.provider, *loc)
		}
	}
	if _, err := parse("bogus", json.NewDecoder(strings.NewReader(`{}`))); err == nil {
		t.Error("unknown provider must fail")
	}
}

func TestLocationStaticOverridesAndNone(t *testing.T) {
	loc, err := Location(Options{Provider: ProviderNone, IP: "198.51.100.9", City: "Turin", Country: "Italy", Latitude: 45.07, Longitude: 7.69})
	if err != nil {
		t.Fatal(err)
	}
	if loc.IP != "198.51.100.9" || loc.City != "Turin" || loc.Latitude != 45.07 || loc.Longitude != 7.69 {
		t.Errorf("got %+v", *loc)
	}
	if _, err := Location(Options{Provider: "bogus"}); err == nil {
		t.Error("unknown provider must fail")
	}
}
