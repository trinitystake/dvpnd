// SPDX-License-Identifier: Apache-2.0
// Modified from sentinel-official/dvpn-node @ 62bde16 (2024-01-25). See NOTICE.

package status

import (
	"net/http"

	"github.com/cosmos/cosmos-sdk/version"
	"github.com/gin-gonic/gin"

	"github.com/trinitystake/dvpnd/context"
	"github.com/trinitystake/dvpnd/types"
)

// HandlerGetRoot serves GET /, the document current client apps read to learn
// what a node runs: service_type and the advertised inbounds, plus everything
// the legacy /status reported.
func HandlerGetRoot(ctx *context.Context) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, types.NewResponseResult(&ResponseGetRoot{
			ResponseGetStatus: statusResponse(ctx),
			ServiceType:       ctx.ServiceTypeName(),
			ServiceMetadata:   ctx.ServiceMetadata(false),
		}))
	}
}

func HandlerGetStatus(ctx *context.Context) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, types.NewResponseResult(statusResponse(ctx)))
	}
}

func statusResponse(ctx *context.Context) *ResponseGetStatus {
	return &ResponseGetStatus{
		Address: ctx.Address().String(),
		Bandwidth: &Bandwidth{
			Upload:   ctx.Bandwidth().Upload.Int64(),
			Download: ctx.Bandwidth().Download.Int64(),
		},
		Handshake: &Handshake{
			Enable: ctx.Config().Handshake.Enable,
			Peers:  ctx.Config().Handshake.Peers,
		},
		IntervalSetSessions:    ctx.IntervalSetSessions(),
		IntervalUpdateSessions: ctx.IntervalUpdateSessions(),
		IntervalUpdateStatus:   ctx.IntervalUpdateStatus(),
		Location: &Location{
			City:        ctx.Location().City,
			Country:     ctx.Location().Country,
			CountryCode: ctx.Location().CountryCode,
			Latitude:    ctx.Location().Latitude,
			Longitude:   ctx.Location().Longitude,
			Source:      ctx.Location().Source,
		},
		Moniker:        ctx.Moniker(),
		Operator:       ctx.Operator().String(),
		Peers:          ctx.Service().PeerCount(),
		GigabytePrices: ctx.GigabytePrices().String(),
		HourlyPrices:   ctx.HourlyPrices().String(),
		QOS: &QOS{
			MaxPeers: ctx.Config().QOS.MaxPeers,
		},
		Type:    ctx.Service().Type(),
		Version: version.Version,
	}
}
