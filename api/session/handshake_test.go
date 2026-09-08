// SPDX-License-Identifier: Apache-2.0

package session

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"

	v2raytypes "github.com/trinitystake/dvpnd/services/v2ray/types"
	wgtypes "github.com/trinitystake/dvpnd/services/wireguard/types"
)

// signedBody builds a request exactly the way client apps do: base64 of the
// peer-request JSON, and a compact secp256k1 signature over
// 8-byte big-endian id || those JSON bytes.
func signedBody(t *testing.T, priv *secp256k1.PrivKey, id uint64, peer interface{}) *HandshakeBody {
	t.Helper()
	data, err := json.Marshal(peer)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := priv.Sign(append(sdk.Uint64ToBigEndian(id), data...))
	if err != nil {
		t.Fatal(err)
	}
	return &HandshakeBody{
		Data:      base64.StdEncoding.EncodeToString(data),
		ID:        id,
		PubKey:    pubKeyPrefix + base64.StdEncoding.EncodeToString(priv.PubKey().Bytes()),
		Signature: base64.StdEncoding.EncodeToString(sig),
	}
}

func TestVerifyHandshake(t *testing.T) {
	priv := secp256k1.GenPrivKey()
	body := signedBody(t, priv, 59840883, map[string]string{"public_key": "abc"})

	addr, data, err := verifyHandshake(body)
	if err != nil {
		t.Fatal(err)
	}
	if !addr.Equals(sdk.AccAddress(priv.PubKey().Address())) {
		t.Fatalf("address mismatch: %s", addr)
	}
	if string(data) != `{"public_key":"abc"}` {
		t.Fatalf("unexpected data %q", data)
	}

	// Tampered id, data or key must all fail.
	tampered := *body
	tampered.ID++
	if _, _, err := verifyHandshake(&tampered); err == nil {
		t.Error("tampered id accepted")
	}
	tampered = *body
	tampered.Data = base64.StdEncoding.EncodeToString([]byte(`{"public_key":"abd"}`))
	if _, _, err := verifyHandshake(&tampered); err == nil {
		t.Error("tampered data accepted")
	}
	tampered = *body
	tampered.PubKey = pubKeyPrefix + base64.StdEncoding.EncodeToString(secp256k1.GenPrivKey().PubKey().Bytes())
	if _, _, err := verifyHandshake(&tampered); err == nil {
		t.Error("wrong key accepted")
	}
	tampered = *body
	tampered.PubKey = "ed25519:" + tampered.PubKey[len(pubKeyPrefix):]
	if _, _, err := verifyHandshake(&tampered); err == nil {
		t.Error("wrong key type accepted")
	}
}

func TestPeerDataFromRequest(t *testing.T) {
	key, err := wgtypes.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	pub := key.Public()

	data, err := peerDataFromRequest(wgtypes.Type, []byte(`{"public_key":"`+pub.String()+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 32 || base64.StdEncoding.EncodeToString(data) != pub.String() {
		t.Fatalf("wireguard key round trip failed: %x", data)
	}

	if _, err := peerDataFromRequest(wgtypes.Type, []byte(`{"public_key":"not-base64!"}`)); err == nil {
		t.Error("bad wireguard key accepted")
	}

	arrayJSON := `{"uuid":[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16]}`
	data, err = peerDataFromRequest(v2raytypes.Type, []byte(arrayJSON))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 17 || data[0] != 0x01 || data[1] != 1 || data[16] != 16 {
		t.Fatalf("v2ray array uuid: %x", data)
	}

	data, err = peerDataFromRequest(v2raytypes.Type, []byte(`{"uuid":"01020304-0506-0708-090a-0b0c0d0e0f10"}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 17 || data[0] != 0x01 || data[1] != 1 || data[16] != 16 {
		t.Fatalf("v2ray string uuid: %x", data)
	}

	if _, err := peerDataFromRequest(v2raytypes.Type, []byte(`{"uuid":42}`)); err == nil {
		t.Error("bad uuid accepted")
	}
	if _, err := peerDataFromRequest(99, []byte(`{}`)); err == nil {
		t.Error("unknown service accepted")
	}
}

func TestWireguardAddrs(t *testing.T) {
	v4 := []byte{10, 8, 0, 3}
	v6 := []byte{0xfd, 0x86, 0xea, 0x04, 0x11, 0x15, 0, 0, 0, 0, 0, 0, 0, 0, 0, 3}
	zero := make([]byte, 16)

	addrs, err := wireguardAddrs(append(append([]byte{}, v4...), v6...))
	if err != nil {
		t.Fatal(err)
	}
	if len(addrs) != 2 || addrs[0] != "10.8.0.3/32" || addrs[1] != "fd86:ea04:1115::3/128" {
		t.Fatalf("dual-stack peer: got %v", addrs)
	}

	addrs, err = wireguardAddrs(append(append([]byte{}, v4...), zero...))
	if err != nil {
		t.Fatal(err)
	}
	if len(addrs) != 1 || addrs[0] != "10.8.0.3/32" {
		t.Fatalf("ipv4-only peer must yield one address, got %v", addrs)
	}

	if _, err := wireguardAddrs(v4); err == nil {
		t.Error("short peer result accepted")
	}
}
