package store

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
)

func SHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func FileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return SHA256(data), nil
}
