// SPDX-License-Identifier: Apache-2.0
// Modified from sentinel-official/dvpn-node @ 62bde16 (2024-01-25). See NOTICE.

package context

import (
	"net"
	"net/http"
	"net/url"
	"time"

	cmtlog "github.com/cometbft/cometbft/libs/log"
	sdk "github.com/cosmos/cosmos-sdk/types"
	base "github.com/sentinel-official/sentinelhub/v12/types"
	v1base "github.com/sentinel-official/sentinelhub/v12/types/v1"
	"gorm.io/gorm"

	geoiptypes "github.com/trinitystake/dvpnd/libs/geoip/types"
	"github.com/trinitystake/dvpnd/lite"
	"github.com/trinitystake/dvpnd/types"
)

type Context struct {
	bandwidth *v1base.Bandwidth
	client    *lite.Client
	config    *types.Config
	database  *gorm.DB
	handler   http.Handler
	location  *geoiptypes.GeoIPLocation
	logger    cmtlog.Logger
	service   types.Service
}

func NewContext() *Context {
	return &Context{}
}

func (c *Context) WithBandwidth(v *v1base.Bandwidth) *Context        { c.bandwidth = v; return c }
func (c *Context) WithClient(v *lite.Client) *Context                { c.client = v; return c }
func (c *Context) WithConfig(v *types.Config) *Context               { c.config = v; return c }
func (c *Context) WithDatabase(v *gorm.DB) *Context                  { c.database = v; return c }
func (c *Context) WithHandler(v http.Handler) *Context               { c.handler = v; return c }
func (c *Context) WithLocation(v *geoiptypes.GeoIPLocation) *Context { c.location = v; return c }
func (c *Context) WithLogger(v cmtlog.Logger) *Context               { c.logger = v; return c }
func (c *Context) WithService(v types.Service) *Context              { c.service = v; return c }

func (c *Context) Address() base.NodeAddress           { return c.Operator().Bytes() }
func (c *Context) Bandwidth() *v1base.Bandwidth        { return c.bandwidth }
func (c *Context) Client() *lite.Client                { return c.client }
func (c *Context) Config() *types.Config               { return c.config }
func (c *Context) Database() *gorm.DB                  { return c.database }
func (c *Context) Handler() http.Handler               { return c.handler }
func (c *Context) IntervalSetSessions() time.Duration  { return c.Config().Node.IntervalSetSessions }
func (c *Context) IntervalUpdateStatus() time.Duration { return c.Config().Node.IntervalUpdateStatus }
func (c *Context) ListenOn() string                    { return c.Config().Node.ListenOn }
func (c *Context) Location() *geoiptypes.GeoIPLocation { return c.location }
func (c *Context) Log() cmtlog.Logger                  { return c.logger }
func (c *Context) Moniker() string                     { return c.Config().Node.Moniker }
func (c *Context) Operator() sdk.AccAddress            { return c.client.FromAddress() }
func (c *Context) RemoteURL() string                   { return c.Config().Node.RemoteURL }
func (c *Context) Service() types.Service              { return c.service }

func (c *Context) IntervalUpdateSessions() time.Duration {
	return c.Config().Node.IntervalUpdateSessions
}

func (c *Context) IPv4Address() net.IP {
	addr := c.Config().Node.IPv4Address
	if addr == "" {
		addr = c.Location().IP
	}

	return net.ParseIP(addr).To4()
}

func (c *Context) GigabytePrices() v1base.Prices {
	prices, err := types.ParsePrices(c.Config().Node.GigabytePrices)
	if err != nil {
		panic(err)
	}

	return prices
}

func (c *Context) HourlyPrices() v1base.Prices {
	prices, err := types.ParsePrices(c.Config().Node.HourlyPrices)
	if err != nil {
		panic(err)
	}

	return prices
}

// RemoteAddrs returns the host:port form of remote_url, which is what the chain
// stores in a node's remote_addrs (v3). Clients prepend the scheme themselves.
func (c *Context) RemoteAddrs() []string {
	u, err := url.Parse(c.RemoteURL())
	if err != nil {
		panic(err)
	}

	return []string{u.Host}
}
