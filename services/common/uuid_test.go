// SPDX-License-Identifier: Apache-2.0

package common

import (
	"encoding/json"
	"testing"
)

func TestUUIDFromJSON(t *testing.T) {
	want := [UUIDLen]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	id, err := UUIDFromJSON(json.RawMessage(`[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16]`))
	if err != nil || id != want {
		t.Fatalf("array form: %v %x", err, id)
	}

	id, err = UUIDFromJSON(json.RawMessage(`"01020304-0506-0708-090a-0b0c0d0e0f10"`))
	if err != nil || id != want {
		t.Fatalf("string form: %v %x", err, id)
	}

	if FormatUUID(want) != "01020304-0506-0708-090a-0b0c0d0e0f10" {
		t.Fatalf("format: %s", FormatUUID(want))
	}

	for _, bad := range []string{`42`, `"nope"`, `[1,2,3]`, `"01020304-0506-0708-090a-0b0c0d0e0f1g"`, ``} {
		if _, err := UUIDFromJSON(json.RawMessage(bad)); err == nil {
			t.Errorf("%s accepted", bad)
		}
	}
}
