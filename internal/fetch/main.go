package fetch

import (
	"context"
	"errors"
	"fmt"
	"github.com/hashicorp/go-retryablehttp"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
)

// One takes a request string for any supported source (local file, url or
// stdin) and handles it with a supplied processing callback.
func One(ctx context.Context, request string, process func(string, io.ReadSeeker) error) error {
	// If the requested input is arriving from a location that does not
	// originate on the machine where memorybox is running (e.g. a user
	// instructing memorybox to fetch a URL) the fetch function will store the
	// data in a temporary file. This ensures the  content can be be read
	// multiple times if needed. For example the put function reads a file once
	// for hashing, once to check to see if it contains metadata and again to
	// actually send it  to the store).
	if ctx.Err() != nil {
		return ctx.Err()
	}
	file, deleteOnClose, fetchErr := new(ctx).fetch(request)
	if fetchErr != nil {
		return fetchErr
	}
	// In cases where a temp file was created by fetching, `deleteWhenDone` will
	// be true. This ensures the underlying temp file is removed.
	if deleteOnClose {
		defer file.Close()
		defer os.Remove(file.Name())
	}
	return process(request, file)
}

// Many does the same thing as One but with gated concurrency.
func Many(
	ctx context.Context,
	requests []string,
	concurrency int,
	process func(int, string, io.ReadSeeker) error,
) error {
	sem := semaphore.NewWeighted(int64(concurrency))
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		for index, item := range requests {
			index, item := index, item // https://golang.org/doc/faq#closures_and_goroutines
			if err := sem.Acquire(egCtx, 1); err != nil {
				return err
			}
			eg.Go(func() error {
				defer sem.Release(1)
				return One(egCtx, item, func(request string, src io.ReadSeeker) error {
					return process(index, request, src)
				})
			})
		}
		return nil
	})
	return eg.Wait()
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
		os.Remove(file.Name())
		return nil, false, copyErr
	}
	file.Seek(0, io.SeekStart)
	return file, true, nil
}
