package status

import (
	"github.com/gin-gonic/gin"

	"github.com/trinitystake/dvpnd/context"
)

func RegisterRoutes(ctx *context.Context, r gin.IRouter) {
	r.GET("/status", HandlerGetStatus(ctx))
}
