package cli

import (
	"errors"
	"github.com/tkellen/memorybox/pkg/test"
	"testing"
)

func TestAggregateErrorStrings(t *testing.T) {
	var errs []string
	errs = aggregateErrorStrings(nil, errs)
	if len(errs) != 0 {
		t.Fatal("expected not to aggregate nil error")
	}
	errString := "test"
	errs = aggregateErrorStrings(errors.New(errString), errs)
	if len(errs) != 1 {
		t.Fatal("expected to aggregate one error")
	}
	if errs[0] != errString {
		t.Fatalf("expected to capture error string %s, got %s", errString, errs[0])
	}
}

func TestHash(t *testing.T) {
	expected := "sha256-9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	actual, err := hash(test.GoodReadCloser([]byte("test")))
	if err != nil {
		t.Fatal(err)
	}
	if expected != actual {
		t.Fatalf("expected %s, got %s", expected, actual)
	}
	result, err := hash(test.TimeoutReadCloser([]byte("test")))
	if result != "" {
		t.Fatalf("expected empty result on bad reader, got %s", result)
	}
	if err == nil {
		t.Fatal("expected error on bad reader")
	}
}

func TestInputOnStdin(t *testing.T) {
	if inputIsStdin("whatever") {
		t.Fatal("expected false")
	}
	if !inputIsStdin("-") {
		t.Fatal("expected true")
	}
}

func TestInputIsURL(t *testing.T) {
	if !inputIsURL("http://") {
		t.Fatal("expected true")
	}
	if !inputIsURL("https://") {
		t.Fatal("expected true")
	}
	if inputIsURL("http") {
		t.Fatal("expected false")
	}
}
