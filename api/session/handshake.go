// SPDX-License-Identifier: Apache-2.0

package session

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/gin-gonic/gin"
	"github.com/v2fly/v2ray-core/v5/common/uuid"

	"github.com/trinitystake/dvpnd/context"
	v2raytypes "github.com/trinitystake/dvpnd/services/v2ray/types"
	wgtypes "github.com/trinitystake/dvpnd/services/wireguard/types"
	"github.com/trinitystake/dvpnd/types"
)

// HandshakeBody is what current client apps POST to a node's root path. The
// client proves it holds the session's account key by signing the session id
// (8 bytes, big-endian) followed by the exact JSON bytes of the peer request,
// and sends the compressed secp256k1 public key alongside — so, unlike the
// legacy endpoint, the account needs no prior on-chain transaction.
type HandshakeBody struct {
	Data      string `json:"data"`
	ID        uint64 `json:"id"`
	PubKey    string `json:"pub_key"`
	Signature string `json:"signature"`
}

// HandshakeResult is the result envelope of a successful handshake: the
// service-specific configuration as base64 JSON, and the hosts the client can
// reach the node on.
type HandshakeResult struct {
	Data  string   `json:"data"`
	Addrs []string `json:"addrs"`
}

const pubKeyPrefix = "secp256k1:"

// verifyHandshake authenticates the body and returns the account address it
// was signed by together with the raw peer-request JSON.
func verifyHandshake(body *HandshakeBody) (sdk.AccAddress, []byte, error) {
	if !strings.HasPrefix(body.PubKey, pubKeyPrefix) {
		return nil, nil, errors.New("pub_key must be secp256k1")
	}

	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(body.PubKey, pubKeyPrefix))
	if err != nil {
		return nil, nil, fmt.Errorf("invalid pub_key: %w", err)
	}
	if len(raw) != secp256k1.PubKeySize {
		return nil, nil, fmt.Errorf("pub_key must be %d bytes, got %d", secp256k1.PubKeySize, len(raw))
	}

	data, err := base64.StdEncoding.DecodeString(body.Data)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid data: %w", err)
	}

	signature, err := base64.StdEncoding.DecodeString(body.Signature)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid signature: %w", err)
	}

	if body.ID == 0 {
		return nil, nil, errors.New("id cannot be zero")
	}

	var (
		pubKey  = &secp256k1.PubKey{Key: raw}
		message = append(sdk.Uint64ToBigEndian(body.ID), data...)
	)

	if !pubKey.VerifySignature(message, signature) {
		return nil, nil, errors.New("signature does not verify")
	}

	return sdk.AccAddress(pubKey.Address()), data, nil
}

// peerDataFromRequest turns the client's JSON peer request into the bytes the
// VPN service's AddPeer expects: the 32-byte WireGuard public key, or the
// proxy byte followed by the 16-byte UUID for V2Ray (VMess). V2Ray clients send
// the UUID either as a 16-element byte array or as its canonical string.
func peerDataFromRequest(serviceType uint64, data []byte) ([]byte, error) {
	switch serviceType {
	case wgtypes.Type:
		var req struct {
			PublicKey string `json:"public_key"`
		}
		if err := json.Unmarshal(data, &req); err != nil {
			return nil, fmt.Errorf("invalid wireguard peer request: %w", err)
		}

		key, err := wgtypes.KeyFromString(req.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("invalid public_key: %w", err)
		}

		return key.Bytes(), nil

	case v2raytypes.Type:
		var req struct {
			UUID json.RawMessage `json:"uuid"`
		}
		if err := json.Unmarshal(data, &req); err != nil {
			return nil, fmt.Errorf("invalid v2ray peer request: %w", err)
		}

		var id uuid.UUID
		var asBytes []byte
		if err := json.Unmarshal(req.UUID, &asBytes); err == nil && len(asBytes) == 16 {
			copy(id[:], asBytes)
		} else {
			var asString string
			if err := json.Unmarshal(req.UUID, &asString); err != nil {
				return nil, errors.New("uuid must be a 16-byte array or a string")
			}
			parsed, err := uuid.ParseString(asString)
			if err != nil {
				return nil, fmt.Errorf("invalid uuid: %w", err)
			}
			id = parsed
		}

		return append([]byte{v2raytypes.Proxy(0x01).Byte()}, id[:]...), nil
	}

	return nil, fmt.Errorf("unsupported service type %d", serviceType)
}

// buildHandshakeResult renders the service configuration for the client.
func buildHandshakeResult(ctx *context.Context, peer []byte) (*HandshakeResult, error) {
	var payload interface{}

	switch ctx.Service().Type() {
	case wgtypes.Type:
		addrs, err := wireguardAddrs(peer)
		if err != nil {
			return nil, err
		}
		payload = struct {
			Addrs    []string `json:"addrs"`
			Metadata []struct {
				Port      uint16 `json:"port"`
				PublicKey string `json:"public_key"`
			} `json:"metadata"`
		}{
			Addrs: addrs,
			Metadata: []struct {
				Port      uint16 `json:"port"`
				PublicKey string `json:"public_key"`
			}{{Port: ctx.ServicePort(), PublicKey: ctx.WireGuardPublicKey()}},
		}
	case v2raytypes.Type:
		payload = struct {
			Metadata []context.Inbound `json:"metadata"`
		}{Metadata: ctx.ServiceMetadata(true)}
	default:
		return nil, fmt.Errorf("unsupported service type %d", ctx.Service().Type())
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &HandshakeResult{
		Data:  base64.StdEncoding.EncodeToString(data),
		Addrs: nodeAddrs(ctx),
	}, nil
}

// wireguardAddrs turns AddPeer's result (4-byte IPv4 followed by 16-byte IPv6)
// into the tunnel addresses the client configures. An all-zero IPv6 means the
// node runs the tunnel IPv4-only and the client must not get an IPv6 address.
func wireguardAddrs(peer []byte) ([]string, error) {
	if len(peer) != 4+16 {
		return nil, fmt.Errorf("unexpected wireguard peer result length %d", len(peer))
	}
	addrs := []string{net.IP(peer[:4]).String() + "/32"}
	if v6 := net.IP(peer[4:20]); !v6.Equal(net.IPv6zero) {
		addrs = append(addrs, v6.String()+"/128")
	}
	return addrs, nil
}

// nodeAddrs lists the hosts a client may use to reach this node: the public
// IPv4 first, then the remote_url host when it differs (for example a DNS name).
func nodeAddrs(ctx *context.Context) []string {
	addrs := []string{ctx.IPv4Address().String()}
	if u, err := url.Parse(ctx.RemoteURL()); err == nil && u.Hostname() != "" && u.Hostname() != addrs[0] {
		addrs = append(addrs, u.Hostname())
	}

	return addrs
}

// HandlerHandshake serves POST / for current client apps.
func HandlerHandshake(ctx *context.Context) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body HandshakeBody
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, types.NewResponseError(2, err))
			return
		}

		accAddr, data, err := verifyHandshake(&body)
		if err != nil {
			c.JSON(http.StatusBadRequest, types.NewResponseError(4, err))
			return
		}

		peerData, err := peerDataFromRequest(ctx.Service().Type(), data)
		if err != nil {
			c.JSON(http.StatusBadRequest, types.NewResponseError(2, err))
			return
		}

		res, apiErr := admit(ctx, admitRequest{AccAddress: accAddr, ID: body.ID, PeerData: peerData})
		if apiErr != nil {
			c.JSON(apiErr.Status, types.NewResponseError(apiErr.Code, apiErr.Err))
			return
		}

		result, err := buildHandshakeResult(ctx, res.Peer)
		if err != nil {
			c.JSON(http.StatusInternalServerError, types.NewResponseError(10, err))
			return
		}

		c.JSON(http.StatusOK, types.NewResponseResult(result))
	}
}
