package api

import (
	"github.com/gin-gonic/gin"

	"github.com/trinitystake/dvpnd/api/session"
	"github.com/trinitystake/dvpnd/api/status"
	"github.com/trinitystake/dvpnd/context"
)

func RegisterRoutes(ctx *context.Context, r gin.IRouter) {
	session.RegisterRoutes(ctx, r)
	status.RegisterRoutes(ctx, r)
}
