// SPDX-License-Identifier: Apache-2.0
// Modified from sentinel-official/dvpn-node @ 62bde16 (2024-01-25). See NOTICE.

package session

import (
	"fmt"
	"net/http"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/gin-gonic/gin"

	"github.com/trinitystake/dvpnd/v9/context"
	"github.com/trinitystake/dvpnd/v9/types"
)

// HandlerAddSession is the legacy endpoint (POST /accounts/:acc_address/sessions/:id).
// The request is authenticated against the public key the chain holds for the
// account, so the account must have transacted before.
func HandlerAddSession(ctx *context.Context) gin.HandlerFunc {
	return func(c *gin.Context) {
		req, err := NewRequestAddSession(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, types.NewResponseError(2, err))
			return
		}

		account, err := ctx.Client().QueryAccount(req.AccAddress)
		if err != nil {
			c.JSON(http.StatusInternalServerError, types.NewResponseError(4, err))
			return
		}
		if account == nil {
			err = fmt.Errorf("account %s does not exist", req.AccAddress)
			c.JSON(http.StatusNotFound, types.NewResponseError(4, err))
			return
		}
		if account.GetPubKey() == nil {
			err = fmt.Errorf("public key for account %s does not exist", req.AccAddress)
			c.JSON(http.StatusNotFound, types.NewResponseError(4, err))
			return
		}

		if ok := account.GetPubKey().VerifySignature(sdk.Uint64ToBigEndian(req.URI.ID), req.Signature); !ok {
			err = fmt.Errorf("invalid signature %s", req.Signature)
			c.JSON(http.StatusBadRequest, types.NewResponseError(4, err))
			return
		}

		res, apiErr := admit(ctx, admitRequest{
			AccAddress: req.AccAddress,
			ID:         req.URI.ID,
			PeerData:   req.Key,
		})
		if apiErr != nil {
			c.JSON(apiErr.Status, types.NewResponseError(apiErr.Code, apiErr.Err))
			return
		}

		result := append([]byte{}, res.Peer...)
		result = append(result, ctx.IPv4Address()...)
		result = append(result, ctx.Service().Info()...)
		c.JSON(http.StatusCreated, types.NewResponseResult(result))
	}
}
