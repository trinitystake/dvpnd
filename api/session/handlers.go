// SPDX-License-Identifier: Apache-2.0
// Modified from sentinel-official/dvpn-node @ 62bde16 (2024-01-25). See NOTICE.

package session

import (
	"fmt"
	"math"
	"net/http"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/gin-gonic/gin"
	v1base "github.com/sentinel-official/sentinelhub/v12/types/v1"
	nodetypes "github.com/sentinel-official/sentinelhub/v12/x/node/types/v3"
	subscriptiontypes "github.com/sentinel-official/sentinelhub/v12/x/subscription/types/v3"

	"github.com/trinitystake/dvpnd/context"
	"github.com/trinitystake/dvpnd/types"
)

func HandlerAddSession(ctx *context.Context) gin.HandlerFunc {
	return func(c *gin.Context) {
		if ctx.Service().PeerCount() >= ctx.Config().QOS.MaxPeers {
			err := fmt.Errorf("reached maximum peers limit %d", ctx.Config().QOS.MaxPeers)
			c.JSON(http.StatusBadRequest, types.NewResponseError(1, err))
			return
		}

		req, err := NewRequestAddSession(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, types.NewResponseError(2, err))
			return
		}

		item := types.Session{}
		ctx.Database().Model(
			&types.Session{},
		).Where(
			&types.Session{
				ID: req.URI.ID,
			},
		).First(&item)

		if item.ID != 0 {
			err = fmt.Errorf("peer for session %d already exist", req.URI.ID)
			c.JSON(http.StatusBadRequest, types.NewResponseError(3, err))
			return
		}

		item = types.Session{}
		ctx.Database().Model(
			&types.Session{},
		).Where(
			&types.Session{
				Key: req.Body.Key,
			},
		).First(&item)

		if item.ID != 0 {
			err = fmt.Errorf("key %s for service already exist", req.Body.Key)
			c.JSON(http.StatusBadRequest, types.NewResponseError(3, err))
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

		var (
			pubKey = account.GetPubKey()
			msg    = sdk.Uint64ToBigEndian(req.URI.ID)
		)

		if ok := pubKey.VerifySignature(msg, req.Signature); !ok {
			err = fmt.Errorf("invalid signature %s", req.Signature)
			c.JSON(http.StatusBadRequest, types.NewResponseError(4, err))
			return
		}

		session, err := ctx.Client().QuerySession(req.URI.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, types.NewResponseError(5, err))
			return
		}
		if session == nil {
			err = fmt.Errorf("session %d does not exist", req.URI.ID)
			c.JSON(http.StatusNotFound, types.NewResponseError(5, err))
			return
		}
		if !session.GetStatus().Equal(v1base.StatusActive) {
			err = fmt.Errorf("invalid status %s for session %d", session.GetStatus(), session.GetID())
			c.JSON(http.StatusNotFound, types.NewResponseError(5, err))
			return
		}
		if session.GetAccAddress() != req.URI.AccAddress {
			err = fmt.Errorf("account address mismatch; expected %s, got %s", req.URI.AccAddress, session.GetAccAddress())
			c.JSON(http.StatusBadRequest, types.NewResponseError(5, err))
			return
		}
		if session.GetNodeAddress() != ctx.Address().String() {
			err = fmt.Errorf("node address mismatch; expected %s, got %s", ctx.Address(), session.GetNodeAddress())
			c.JSON(http.StatusBadRequest, types.NewResponseError(7, err))
			return
		}

		// remainingBytes is the local cap enforced by the set_sessions job; 0 means
		// no cap. Every session type may carry max_bytes on the chain; plan
		// subscription sessions are additionally bounded by the account's allocation.
		var (
			remainingBytes int64  = 0
			subscriptionID uint64 = 0
		)

		if max := session.GetMaxBytes(); max.IsPositive() {
			diff := max.Sub(session.TotalBytes())
			if !diff.IsPositive() {
				err = fmt.Errorf("session %d has used its %s bytes", session.GetID(), max)
				c.JSON(http.StatusBadRequest, types.NewResponseError(8, err))
				return
			}
			remainingBytes = clampInt64(diff)
		}

		switch s := session.(type) {
		case *nodetypes.Session:
			// pay-per-session with this node; nothing beyond max_bytes to check
		case *subscriptiontypes.Session:
			subscriptionID = s.SubscriptionID

			subscription, err := ctx.Client().QuerySubscription(s.SubscriptionID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, types.NewResponseError(6, err))
				return
			}
			if subscription == nil {
				err = fmt.Errorf("subscription %d does not exist", s.SubscriptionID)
				c.JSON(http.StatusNotFound, types.NewResponseError(6, err))
				return
			}
			if !subscription.Status.Equal(v1base.StatusActive) {
				err = fmt.Errorf("invalid status %s for subscription %d", subscription.Status, subscription.ID)
				c.JSON(http.StatusBadRequest, types.NewResponseError(6, err))
				return
			}

			alloc, err := ctx.Client().QueryAllocation(subscription.ID, req.AccAddress)
			if err != nil {
				c.JSON(http.StatusInternalServerError, types.NewResponseError(8, err))
				return
			}
			if alloc == nil {
				err = fmt.Errorf("allocation %d/%s does not exist", subscription.ID, req.AccAddress)
				c.JSON(http.StatusNotFound, types.NewResponseError(8, err))
				return
			}

			// Count bytes this node has already served on the same allocation but
			// not yet reported to the chain.
			var items []types.Session
			ctx.Database().Model(
				&types.Session{},
			).Where(
				&types.Session{
					Subscription: subscription.ID,
					Address:      req.URI.AccAddress,
				},
			).Find(&items)

			for i := 0; i < len(items); i++ {
				alloc.UtilisedBytes = alloc.UtilisedBytes.Add(sdkmath.NewInt(items[i].Download + items[i].Upload))
			}

			if alloc.UtilisedBytes.GTE(alloc.GrantedBytes) {
				err = fmt.Errorf("invalid allocation; granted bytes %s, utilised bytes %s", alloc.GrantedBytes, alloc.UtilisedBytes)
				c.JSON(http.StatusBadRequest, types.NewResponseError(8, err))
				return
			}

			if left := clampInt64(alloc.GrantedBytes.Sub(alloc.UtilisedBytes)); remainingBytes == 0 || left < remainingBytes {
				remainingBytes = left
			}
		default:
			err = fmt.Errorf("invalid type %T for session %d", s, session.GetID())
			c.JSON(http.StatusBadRequest, types.NewResponseError(7, err))
			return
		}

		// One peer per account per (subscription or node) session set: drop any
		// earlier peer of this account before adding the new one.
		var items []types.Session
		ctx.Database().Model(
			&types.Session{},
		).Where(
			&types.Session{
				Subscription: subscriptionID,
				Address:      req.URI.AccAddress,
			},
		).Find(&items)

		for i := 0; i < len(items); i++ {
			if err = ctx.RemovePeerIfExists(items[i].Key); err != nil {
				c.JSON(http.StatusInternalServerError, types.NewResponseError(9, err))
				return
			}
		}

		result, err := ctx.Service().AddPeer(req.Key)
		if err != nil {
			c.JSON(http.StatusInternalServerError, types.NewResponseError(10, err))
			return
		}
		ctx.Log().Info("Added a new peer", "key", req.Body.Key, "count", ctx.Service().PeerCount())

		ctx.Database().Model(
			&types.Session{},
		).Create(
			&types.Session{
				ID:           req.URI.ID,
				Subscription: subscriptionID,
				Key:          req.Body.Key,
				Address:      req.URI.AccAddress,
				Available:    remainingBytes,
			},
		)

		result = append(result, ctx.IPv4Address()...)
		result = append(result, ctx.Service().Info()...)
		c.JSON(http.StatusCreated, types.NewResponseResult(result))
	}
}

// clampInt64 converts a positive chain integer to int64, saturating at MaxInt64.
func clampInt64(v sdkmath.Int) int64 {
	if v.IsInt64() {
		return v.Int64()
	}

	return math.MaxInt64
}
