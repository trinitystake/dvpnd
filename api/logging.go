// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/trinitystake/dvpnd/context"
)

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
