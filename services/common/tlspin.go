// SPDX-License-Identifier: Apache-2.0

package common

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
)

// CertificatePin returns the hex SHA-256 of the first certificate in a PEM
// file: what clients pin, since a node's certificate is self-signed.
func CertificatePin(path string) (string, error) {
	pemBytes, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	block, _ := pem.Decode(pemBytes)
	if block == nil || block.Type != "CERTIFICATE" {
		return "", fmt.Errorf("no certificate found in %s", path)
	}

	sum := sha256.Sum256(block.Bytes)

	return hex.EncodeToString(sum[:]), nil
}
