// SPDX-License-Identifier: Apache-2.0
// Modified from sentinel-official/dvpn-node @ 62bde16 (2024-01-25). See NOTICE.

package session

import (
	"github.com/gin-gonic/gin"

	"github.com/trinitystake/dvpnd/context"
)

func RegisterRoutes(ctx *context.Context, router gin.IRouter) {
	// Current client apps handshake at the root path.
	router.POST("/", HandlerHandshake(ctx))
	// Legacy endpoint kept for older clients.
	router.POST("/accounts/:acc_address/sessions/:id", HandlerAddSession(ctx))
}
