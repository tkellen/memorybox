package fetch

import (
	"context"
	"errors"
	"fmt"
	"github.com/hashicorp/go-retryablehttp"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
)

// Do takes a request string for any supported source (local file, url or stdin)
// and returns an os.File + a boolean indicating whether the request was a file
// on disk (if it was not, the consumer should delete the file on close).
func Do(ctx context.Context, req string) (file *os.File, deleteOnClose bool, err error) {
	return new(ctx).fetch(req)
}

// sys defines a set of methods for network and disk io. This is an attempt to
// make the thinnest possible abstraction to support achieving 100% test
// coverage without a runtime dependency on a mocking library.
// Note:
// There appears to be no coherent way in golang to mock *os.File, nor a virtual
// filesystem implementation available in the standard library. For more info,
// read these links:
// https://github.com/golang/go/issues/14106
// https://github.com/golang/go/issues/21592
type sys struct {
	Get      func(url string) (*http.Response, error)
	Open     func(string) (*os.File, error)
	Stdin    io.ReadCloser
	TempFile func(string, string) (*os.File, error)
	TempDir  string
}

var errBadRequest = errors.New("bad request")

func new(ctx context.Context) *sys {
	return &sys{
		Get: func(url string) (*http.Response, error) {
			client := retryablehttp.NewClient()
			client.Logger = log.New(ioutil.Discard, "", 0)
			request, err := retryablehttp.NewRequest("GET", url, nil)
			if err != nil {
				return nil, err
			}
			return client.Do(request.WithContext(ctx))
		},
		Open:     os.Open,
		Stdin:    os.Stdin,
		TempFile: ioutil.TempFile,
		TempDir:  os.TempDir(),
	}
}

func (sys *sys) fetch(req string) (*os.File, bool, error) {
	// If the input string is determined to represent stdin (per common
	// convention ("-") is used for this, buffer it to a temporary file.
	if req == "-" {
		return sys.bufferToTempFile(sys.Stdin)
	}
	// If the input string is determined to be a URL, attempt a http request to
	// get the contents and buffer it to a temporary file.
	if u, err := url.Parse(req); err == nil && u.Scheme != "" && u.Host != "" {
		resp, getErr := sys.Get(req)
		if getErr != nil {
			return nil, false, fmt.Errorf("%w: %s", errBadRequest, getErr)
		}
		if !(resp.StatusCode >= 200 && resp.StatusCode <= 299) {
			return nil, false, fmt.Errorf("%w: %d", errBadRequest, resp.StatusCode)
		}
		return sys.bufferToTempFile(resp.Body)
	}
	file, err := sys.Open(req)
	return file, false, err
}

// bufferToTempFile does just what it sounds like.
func (sys *sys) bufferToTempFile(reader io.Reader) (*os.File, bool, error) {
	file, err := sys.TempFile(sys.TempDir, "*")
	if err != nil {
		return nil, false, err
	}
	_, copyErr := io.Copy(file, reader)
	if copyErr != nil {
		return nil, false, copyErr
	}
	file.Seek(0, io.SeekStart)
	return file, true, nil
}
