// SPDX-License-Identifier: Apache-2.0
// Modified from sentinel-official/dvpn-node @ 62bde16 (2024-01-25). See NOTICE.

package api

import (
	"github.com/gin-gonic/gin"

	"github.com/trinitystake/dvpnd/api/session"
	"github.com/trinitystake/dvpnd/api/status"
	"github.com/trinitystake/dvpnd/context"
)

func RegisterRoutes(ctx *context.Context, r gin.IRouter) {
	r.Use(serverHeader())
	r.Use(logRefusals(ctx))
	session.RegisterRoutes(ctx, r)
	status.RegisterRoutes(ctx, r)
}
