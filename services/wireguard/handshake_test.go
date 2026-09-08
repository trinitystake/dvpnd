// SPDX-License-Identifier: Apache-2.0

package wireguard

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"testing"

	wgtypes "github.com/trinitystake/dvpnd/services/wireguard/types"
)

func TestParsePeerRequest(t *testing.T) {
	key, err := wgtypes.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	pub := key.Public()

	s := NewVariant(Default, nil)
	data, err := s.ParsePeerRequest([]byte(`{"public_key":"` + pub.String() + `"}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 32 || base64.StdEncoding.EncodeToString(data) != pub.String() {
		t.Fatalf("key round trip failed: %x", data)
	}

	if _, err := s.ParsePeerRequest([]byte(`{"public_key":"not-base64!"}`)); err == nil {
		t.Error("bad key accepted")
	}
}

func TestHandshakePayload(t *testing.T) {
	s := NewVariant(Default, nil)
	binary.BigEndian.PutUint16(s.info, 51820)
	s.info[2] = 0xab

	v4 := []byte{10, 8, 0, 3}
	v6 := []byte{0xfd, 0x86, 0xea, 0x04, 0x11, 0x15, 0, 0, 0, 0, 0, 0, 0, 0, 0, 3}
	zero := make([]byte, 16)

	payload, err := s.HandshakePayload(append(append([]byte{}, v4...), v6...))
	if err != nil {
		t.Fatal(err)
	}
	out, _ := json.Marshal(payload)
	want := `{"addrs":["10.8.0.3/32","fd86:ea04:1115::3/128"],"metadata":[{"port":51820,"public_key":"` + s.PublicKey() + `"}]}`
	if string(out) != want {
		t.Fatalf("dual-stack payload:\n got %s\nwant %s", out, want)
	}

	payload, err = s.HandshakePayload(append(append([]byte{}, v4...), zero...))
	if err != nil {
		t.Fatal(err)
	}
	if got := payload.(HandshakePayloadData).Addrs; len(got) != 1 || got[0] != "10.8.0.3/32" {
		t.Fatalf("ipv4-only peer must yield one address, got %v", got)
	}

	if _, err := s.HandshakePayload([]byte{1, 2, 3}); err == nil {
		t.Error("short result accepted")
	}

	if md := s.Metadata(true); len(md) != 1 || md[0].Port != 51820 || md[0].TLSPin != "" {
		t.Fatalf("metadata: %+v", md)
	}
	if s.Name() != "wireguard" {
		t.Fatalf("name: %s", s.Name())
	}
}
