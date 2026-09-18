// SPDX-License-Identifier: Apache-2.0

package types

import (
	"os"
	"strconv"
)

func itoa(v interface{}) string {
	switch n := v.(type) {
	case uint16:
		return strconv.FormatUint(uint64(n), 10)
	case uint32:
		return strconv.FormatUint(uint64(n), 10)
	}

	return ""
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
