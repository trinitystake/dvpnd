package session

import (
	"github.com/gin-gonic/gin"

	"github.com/trinitystake/dvpnd/context"
)

func RegisterRoutes(ctx *context.Context, router gin.IRouter) {
	router.POST("/accounts/:acc_address/sessions/:id", HandlerAddSession(ctx))
}
