// SPDX-License-Identifier: Apache-2.0

package v2ray

import (
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestParsePeerRequest(t *testing.T) {
	s := NewV2Ray()

	data, err := s.ParsePeerRequest([]byte(`{"uuid":[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 17 || data[0] != 0x01 || data[1] != 1 || data[16] != 16 {
		t.Fatalf("array uuid: %x", data)
	}

	data, err = s.ParsePeerRequest([]byte(`{"uuid":"01020304-0506-0708-090a-0b0c0d0e0f10"}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 17 || data[0] != 0x01 || data[1] != 1 || data[16] != 16 {
		t.Fatalf("string uuid: %x", data)
	}

	if _, err := s.ParsePeerRequest([]byte(`{"uuid":42}`)); err == nil {
		t.Error("bad uuid accepted")
	}
}

func TestMetadataAndPayload(t *testing.T) {
	s := NewV2Ray()
	binary.BigEndian.PutUint16(s.info, 8443)
	s.info[2] = 0x01 // tcp
	s.info[3] = 0    // no TLS

	md := s.Metadata(true)
	if len(md) != 1 || md[0].Port != 8443 || md[0].ProxyProtocol != 2 || md[0].TransportProtocol != 1 ||
		md[0].TransportSecurity != 1 || md[0].TLSPin != "" {
		t.Fatalf("plain inbound: %+v", md)
	}

	s.info[3] = 1
	s.tlsPin = "abc"
	if md := s.Metadata(false); md[0].TransportSecurity != 2 || md[0].TLSPin != "" {
		t.Fatalf("public listing must omit the pin: %+v", md)
	}

	payload, err := s.HandshakePayload(nil)
	if err != nil {
		t.Fatal(err)
	}
	out, _ := json.Marshal(payload)
	want := `{"metadata":[{"port":8443,"proxy_protocol":2,"transport_protocol":1,"transport_security":2,"tls_pin":"abc"}]}`
	if string(out) != want {
		t.Fatalf("payload:\n got %s\nwant %s", out, want)
	}
	if s.Name() != "v2ray" {
		t.Fatalf("name: %s", s.Name())
	}
}
