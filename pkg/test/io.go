package test

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"strings"
	"testing/iotest"
)

// TimeoutReadCloser produces an io.ReadCloser that will timeout when read.
func TimeoutReadCloser(input []byte) io.ReadCloser {
	return ioutil.NopCloser(iotest.TimeoutReader(bytes.NewReader(input)))
}

// GoodReadCloser produces an io.ReadCloser that functions properly.
func GoodReadCloser(input []byte) io.ReadCloser {
	return ioutil.NopCloser(bytes.NewReader(input))
}

// TempDir creates a temporary directory that matches the function name of
// the caller. For integration testing with actual disk io that needs to be
// isolated per-test.
func TempDir() string {
	pc := make([]uintptr, 10)
	runtime.Callers(2, pc)
	parts := strings.Split(runtime.FuncForPC(pc[0]).Name(), ".")
	path := path.Join(os.TempDir(), parts[len(parts)-1])
	os.RemoveAll(path)
	return path
}
