package commands_test

import (
	"bytes"
	"github.com/tkellen/memorybox/commands"
	"testing"
	"testing/iotest"
)

func TestSha256(t *testing.T) {
	input := []byte("test")
	expected := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08-sha256"
	expectedSize := int64(len(input))
	actual, actualSize, goodErr := commands.Sha256(bytes.NewReader(input))
	if goodErr != nil {
		t.Fatal(goodErr)
	}
	if expected != actual {
		t.Fatalf("expected %s, got %s", expected, actual)
	}
	if expectedSize != actualSize {
		t.Fatalf("expected size %d, got %d", expectedSize, actualSize)
	}
	_, _, err := commands.Sha256(iotest.TimeoutReader(bytes.NewReader([]byte("test"))))
	if err == nil {
		t.Fatal("expected error on bad reader")
	}
}
