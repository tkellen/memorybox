package cli

import (
	"encoding/hex"
	"github.com/minio/sha256-simd"
	"io"
	"strings"
)

// aggregateErrors does just what it sounds like.
func aggregateErrorStrings(err error, errs []string) []string {
	if err != nil {
		return append(errs, err.Error())
	}
	return errs
}

// hash computes a sha256 message digest for a provided io.ReadCloser.
func hash(source io.ReadCloser) (string, error) {
	defer source.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, source); err != nil {
		return "", err
	}
	return "sha256-" + hex.EncodeToString(hash.Sum(nil)), nil
}

// inputIsStdin determines if a provided input points to data arriving over
// stdin. Per common convention, we recognize a single dash ("-") as meaning
// this.
func inputIsStdin(input string) bool {
	return input == "-"
}

// inputIsURL determines if we can find our input by making a http request.
func inputIsURL(input string) bool {
	return strings.HasPrefix(input, "http://") ||
		strings.HasPrefix(input, "https://")
}
