package fetch

import (
	"context"
	"errors"
	"fmt"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/tkellen/memorybox/pkg/file"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// Do eases the process of locating data referenced at the command line. It
// will automatically detect bits arriving via stdin, make requests for urls,
// and expand local directories recursively to find all of their files. The
// process callback is invoked once for each item found.
func Do(
	ctx context.Context,
	requests []string,
	concurrency int,
	process func(context.Context, int, *file.File) error,
) error {
	// Ensure any requests which are directories are fully traversed and
	// converted to full file listings.
	expandedRequests := new(ctx).expand(requests)
	sem := semaphore.NewWeighted(int64(concurrency))
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		for index, item := range expandedRequests {
			index, item := index, item // https://golang.org/doc/faq#closures_and_goroutines
			if err := sem.Acquire(egCtx, 1); err != nil {
				return err
			}
			eg.Go(func() error {
				defer sem.Release(1)
				// If the requested input is arriving from a location that does
				// not originate on the machine where memorybox is running (e.g.
				// a user instructing memorybox to fetch a URL), fetch stores
				// the data in a temporary file on local disk. This ensures the
				// content can be be read multiple times if needed.
				f, deleteOnClose, fetchErr := new(egCtx).fetch(item)
				if fetchErr != nil {
					return fetchErr
				}
				// If a temp file was created to buffer the file for multiple
				// reads, delete it after we are done.
				if deleteOnClose {
					defer f.Close()
					// Within this function the body of the file.File is
					// always an os.File.
					defer os.Remove(f.Body.(*os.File).Name())
				}
				return process(egCtx, index, f)
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
	Stat     func(string) (os.FileInfo, error)
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
			request, _ := retryablehttp.NewRequest("GET", url, nil)
			return client.Do(request.WithContext(ctx))
		},
		Open:     os.Open,
		Stat:     os.Stat,
		Stdin:    os.Stdin,
		TempFile: ioutil.TempFile,
		TempDir:  os.TempDir(),
	}
}

func (sys *sys) expand(requests []string) []string {
	expanded := map[string]struct{}{}
	var result []string
	for _, item := range requests {
		if info, err := os.Stat(item); err == nil && info.IsDir() {
			filepath.Walk(item, func(path string, f os.FileInfo, err error) error {
				if !f.IsDir() {
					expanded[path] = struct{}{}
				}
				return nil
			})
		} else {
			expanded[item] = struct{}{}
		}
	}
	for f := range expanded {
		result = append(result, f)
	}
	return result
}

func (sys *sys) fetch(src string) (*file.File, bool, error) {
	var f *file.File
	var err error
	deleteOnClose := true
	if src == "-" {
		// If the input string is determined to represent stdin (per common
		// convention ("-") is used for this, buffer it to a temporary file.
		f, err = sys.fileFromStdin()
	} else if u, ok := url.Parse(src); ok == nil && u.Scheme != "" && u.Host != "" {
		// If the input string is determined to be a URL, attempt a http request
		// to get the contents and buffer it to a temporary file.
		f, err = sys.fileFromURL(src)
	} else {
		// Final case is trying to fetch a file from local disk.
		f, err = sys.fileFromDisk(src)
		deleteOnClose = false
	}
	return f, deleteOnClose, err
}

func (sys *sys) fileFromStdin() (*file.File, error) {
	temp, tempErr := sys.bufferToTempFile(sys.Stdin)
	if tempErr != nil {
		return nil, tempErr
	}
	return file.NewSha256("stdin", temp, time.Now())
}

func (sys *sys) fileFromURL(source string) (*file.File, error) {
	resp, getErr := sys.Get(source)
	if getErr != nil {
		return nil, fmt.Errorf("%w: %s", errBadRequest, getErr)
	}
	if !(resp.StatusCode >= 200 && resp.StatusCode <= 299) {
		return nil, fmt.Errorf("%w: %d", errBadRequest, resp.StatusCode)
	}
	lastModified, err := http.ParseTime(resp.Header.Get("Last-Modified"))
	if err != nil {
		lastModified = time.Now()
	}
	temp, tempErr := sys.bufferToTempFile(resp.Body)
	if tempErr != nil {
		return nil, tempErr
	}
	return file.NewSha256(source, temp, lastModified)
}

func (sys *sys) fileFromDisk(source string) (*file.File, error) {
	f, openErr := sys.Open(source)
	if openErr != nil {
		return nil, openErr
	}
	fileInfo, statErr := sys.Stat(f.Name())
	if statErr != nil {
		return nil, statErr
	}
	return file.NewSha256(source, f, fileInfo.ModTime())
}

func (sys *sys) bufferToTempFile(reader io.Reader) (*os.File, error) {
	f, err := sys.TempFile(sys.TempDir, "*")
	if err != nil {
		return nil, err
	}
	_, copyErr := io.Copy(f, reader)
	if copyErr != nil {
		os.Remove(f.Name())
		return nil, copyErr
	}
	f.Seek(0, io.SeekStart)
	return f, nil
}
