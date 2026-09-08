// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cmtlog "github.com/cometbft/cometbft/libs/log"
	"github.com/gin-gonic/gin"

	"github.com/trinitystake/dvpnd/context"
)

func TestLogRefusals(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var buf bytes.Buffer
	ctx := context.NewContext().WithLogger(cmtlog.NewTMLogger(&buf))

	r := gin.New()
	r.Use(logRefusals(ctx))
	r.GET("/ok", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"success": true}) })
	r.POST("/", func(c *gin.Context) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": gin.H{"code": 2, "message": "signature does not verify"}})
	})

	serve := func(method, path string) {
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("User-Agent", "probe/1.0")
		req.RemoteAddr = "203.0.113.9:4242"
		r.ServeHTTP(httptest.NewRecorder(), req)
	}

	serve(http.MethodGet, "/ok")
	if buf.Len() != 0 {
		t.Fatalf("a successful request must not be logged: %s", buf.String())
	}

	serve(http.MethodPost, "/")
	out := buf.String()
	for _, want := range []string{"Request refused", "method=POST", "path=/", "status=400", "client=203.0.113.9", "agent=probe/1.0", "signature does not verify"} {
		if !strings.Contains(out, want) {
			t.Errorf("log lacks %q: %s", want, out)
		}
	}

	buf.Reset()
	serve(http.MethodGet, "/nowhere")
	if !strings.Contains(buf.String(), "status=404") || !strings.Contains(buf.String(), "path=/nowhere") {
		t.Fatalf("unknown path must be logged: %s", buf.String())
	}
}
