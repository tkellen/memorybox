package commands

import (
	"encoding/hex"
	hash "github.com/minio/sha256-simd"
	"io"
)

// Sha256 computes a sha256 message digest for a provided io.Reader.
func Sha256(source io.Reader) (string, int64, error) {
	hash := hash.New()
	size, err := io.Copy(hash, source)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hash.Sum(nil)) + "-sha256", size, nil
}
