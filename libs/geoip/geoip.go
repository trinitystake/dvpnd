// SPDX-License-Identifier: Apache-2.0
// Modified from sentinel-official/dvpn-node @ 62bde16 (2024-01-25). See NOTICE.

package geoip

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/trinitystake/dvpnd/libs/geoip/types"
)

// Provider names accepted in the [geoip] config section.
const (
	ProviderIPify  = "ipify"  // public IP only; https://www.ipify.org — open source, no stated limits
	ProviderIPAPI  = "ip-api" // IP + city/country/coordinates; free tier is for non-commercial use only
	ProviderIPInfo = "ipinfo" // IP + country (free tier); token required
	ProviderNone   = "none"   // no request; IP comes from node.ipv4_address
)

var defaultURLs = map[string]string{
	ProviderIPify:  "https://api.ipify.org?format=json",
	ProviderIPAPI:  "http://ip-api.com/json", // the free tier does not serve HTTPS
	ProviderIPInfo: "https://ipinfo.io/json",
}

// Options selects the lookup service and any operator-supplied values.
type Options struct {
	Provider string
	URL      string // optional endpoint override for the provider
	Token    string // optional bearer token (ipinfo)
	IP       string // used as the address when Provider is "none"

	// Static values override whatever the provider returned, when set.
	City      string
	Country   string
	Latitude  float64
	Longitude float64
}

// Location discovers the node's public address and, depending on the provider,
// its location, then applies the static overrides from opts.
func Location(opts Options) (*types.GeoIPLocation, error) {
	var (
		loc *types.GeoIPLocation
		err error
	)

	switch opts.Provider {
	case ProviderNone:
		loc = &types.GeoIPLocation{IP: opts.IP}
	case ProviderIPify, ProviderIPAPI, ProviderIPInfo:
		loc, err = fetch(opts)
	default:
		err = fmt.Errorf("unknown geoip provider %q", opts.Provider)
	}
	if err != nil {
		return nil, err
	}

	if opts.City != "" {
		loc.City = opts.City
	}
	if opts.Country != "" {
		loc.Country = opts.Country
	}
	if opts.Latitude != 0 || opts.Longitude != 0 {
		loc.Latitude, loc.Longitude = opts.Latitude, opts.Longitude
	}

	return loc, nil
}

func fetch(opts Options) (*types.GeoIPLocation, error) {
	url := opts.URL
	if url == "" {
		url = defaultURLs[opts.Provider]
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if opts.Token != "" {
		req.Header.Set("Authorization", "Bearer "+opts.Token)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("geoip provider %s returned HTTP %d", opts.Provider, resp.StatusCode)
	}

	return parse(opts.Provider, json.NewDecoder(resp.Body))
}

// parse decodes one provider's response shape into the common location type.
func parse(provider string, dec *json.Decoder) (*types.GeoIPLocation, error) {
	switch provider {
	case ProviderIPify:
		var body struct {
			IP string `json:"ip"`
		}
		if err := dec.Decode(&body); err != nil {
			return nil, err
		}
		return &types.GeoIPLocation{IP: body.IP}, nil

	case ProviderIPAPI:
		var body struct {
			City      string  `json:"city"`
			Country   string  `json:"country"`
			IP        string  `json:"query"`
			Latitude  float64 `json:"lat"`
			Longitude float64 `json:"lon"`
		}
		if err := dec.Decode(&body); err != nil {
			return nil, err
		}
		return &types.GeoIPLocation{
			City: body.City, Country: body.Country, IP: body.IP,
			Latitude: body.Latitude, Longitude: body.Longitude,
		}, nil

	case ProviderIPInfo:
		var body struct {
			IP      string `json:"ip"`
			City    string `json:"city"`
			Country string `json:"country"`
			Loc     string `json:"loc"` // "lat,lon"
		}
		if err := dec.Decode(&body); err != nil {
			return nil, err
		}
		loc := &types.GeoIPLocation{IP: body.IP, City: body.City, Country: body.Country}
		if parts := strings.SplitN(body.Loc, ",", 2); len(parts) == 2 {
			loc.Latitude, _ = strconv.ParseFloat(parts[0], 64)
			loc.Longitude, _ = strconv.ParseFloat(parts[1], 64)
		}
		return loc, nil
	}

	return nil, fmt.Errorf("unknown geoip provider %q", provider)
}
