// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"net/http"

	"github.com/cosmos/cosmos-sdk/version"
	"github.com/gin-gonic/gin"

	"github.com/trinitystake/dvpnd/v9/context"
	"github.com/trinitystake/dvpnd/v9/types"
)

// serverHeader names the node software on every response, the way any HTTP
// server does, so anyone looking at the node (an aggregator, a client
// developer, curl) can tell which implementation and version answered.
func serverHeader() gin.HandlerFunc {
	value := types.AppName + "/" + version.Version

	return func(c *gin.Context) {
		c.Header("Server", value)
		c.Next()
	}
}

// bodyCapture keeps a copy of a small response body so a refusal can be
// logged with the reason the client was given.
type bodyCapture struct {
	gin.ResponseWriter
	body bytes.Buffer
}

func (w *bodyCapture) Write(b []byte) (int, error) {
	if w.body.Len() < 512 {
		w.body.Write(b[:min(len(b), 512-w.body.Len())])
	}

	return w.ResponseWriter.Write(b)
}

// logRefusals logs every request the node answers with an error status:
// who asked, what for, and the reason. A node operator otherwise has no way
// to see why a client or an aggregator's probe was turned away.
func logRefusals(ctx *context.Context) gin.HandlerFunc {
	return func(c *gin.Context) {
		w := &bodyCapture{ResponseWriter: c.Writer}
		c.Writer = w

		c.Next()

		status := c.Writer.Status()
		if status < http.StatusBadRequest {
			ctx.Log().Debug("Request served",
				"method", c.Request.Method,
				"path", c.Request.URL.Path,
				"status", status,
				"client", c.ClientIP(),
				"agent", c.Request.UserAgent(),
			)

			return
		}

		ctx.Log().Error("Request refused",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", status,
			"client", c.ClientIP(),
			"agent", c.Request.UserAgent(),
			"reply", w.body.String(),
		)
	}
}
