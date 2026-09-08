// SPDX-License-Identifier: Apache-2.0

// Package common holds what every protocol service shares: peer-request
// parsing helpers, the TLS certificate pin, and the generic config CLI.
package common

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// UUIDLen is the length of a binary UUID.
const UUIDLen = 16

// UUIDFromJSON decodes the "uuid" value of a peer request, which client apps
// send either as a 16-element byte array or as the canonical string form.
func UUIDFromJSON(raw json.RawMessage) ([UUIDLen]byte, error) {
	var id [UUIDLen]byte

	if len(raw) == 0 {
		return id, errors.New("uuid is missing")
	}

	var asBytes []byte
	if err := json.Unmarshal(raw, &asBytes); err == nil {
		if len(asBytes) != UUIDLen {
			return id, fmt.Errorf("uuid must be %d bytes, got %d", UUIDLen, len(asBytes))
		}
		copy(id[:], asBytes)

		return id, nil
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err != nil {
		return id, errors.New("uuid must be a 16-byte array or a string")
	}

	return ParseUUID(asString)
}

// ParseUUID parses the canonical 8-4-4-4-12 hex form.
func ParseUUID(s string) ([UUIDLen]byte, error) {
	var id [UUIDLen]byte

	parts := strings.Split(s, "-")
	if len(parts) != 5 || len(parts[0]) != 8 || len(parts[1]) != 4 ||
		len(parts[2]) != 4 || len(parts[3]) != 4 || len(parts[4]) != 12 {
		return id, fmt.Errorf("invalid uuid %q", s)
	}

	decoded, err := hex.DecodeString(strings.Join(parts, ""))
	if err != nil {
		return id, fmt.Errorf("invalid uuid %q: %w", s, err)
	}
	copy(id[:], decoded)

	return id, nil
}

// FormatUUID renders the canonical lower-case string form.
func FormatUUID(id [UUIDLen]byte) string {
	h := hex.EncodeToString(id[:])

	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}
