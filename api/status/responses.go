// SPDX-License-Identifier: Apache-2.0

package status

import (
	"time"

	"github.com/trinitystake/dvpnd/types"
)

type (
	Bandwidth struct {
		Download int64 `json:"download"`
		Upload   int64 `json:"upload"`
	}
	Handshake struct {
		Enable bool   `json:"enable"`
		Peers  uint64 `json:"peers"`
	}
	Location struct {
		City        string  `json:"city"`
		Country     string  `json:"country"`
		CountryCode string  `json:"country_code"` // ISO 3166-1 alpha-2, empty when unknown
		Latitude    float64 `json:"latitude"`
		Longitude   float64 `json:"longitude"`
		Source      string  `json:"source"` // service that answered, "static", or "none"
	}
	QOS struct {
		MaxPeers int `json:"max_peers"`
	}
	ResponseGetStatus struct {
		Address                string        `json:"address"`
		Bandwidth              *Bandwidth    `json:"bandwidth"`
		Handshake              *Handshake    `json:"handshake"`
		IntervalSetSessions    time.Duration `json:"interval_set_sessions"`
		IntervalUpdateSessions time.Duration `json:"interval_update_sessions"`
		IntervalUpdateStatus   time.Duration `json:"interval_update_status"`
		Location               *Location     `json:"location"`
		Moniker                string        `json:"moniker"`
		Operator               string        `json:"operator"`
		Peers                  int           `json:"peers"`
		GigabytePrices         string        `json:"gigabyte_prices"`
		HourlyPrices           string        `json:"hourly_prices"`
		QOS                    *QOS          `json:"qos"`
		Type                   uint64        `json:"type"`
		Version                string        `json:"version"`
	}
)

// ResponseGetRoot is the root document: the legacy status fields plus the
// service description current clients look for.
type ResponseGetRoot struct {
	*ResponseGetStatus
	ServiceType     string          `json:"service_type"`
	ServiceMetadata []types.Inbound `json:"service_metadata"`
}
