// Package archive defines an interface for storage engines that have no
// understanding of memorybox. It also provides higher level crud functions for
// interacting with any storage engine in a memorybox specific way.
package archive

import (
	"context"
	"github.com/tkellen/memorybox/pkg/file"
	"io"
	"log"
	"time"
)

// Logger defines output streams for interacting with archives.
type Logger struct {
	Stdout  *log.Logger
	Stderr  *log.Logger
	Verbose *log.Logger
}

// Store defines a storage engine that can persist and retrieve content.
type Store interface {
	Get(context.Context, string) (*file.File, error)
	Put(context.Context, io.Reader, string, time.Time) error
	Delete(context.Context, string) error
	Search(context.Context, string) (file.List, error)
	Concat(context.Context, int, []string) ([][]byte, error)
	Stat(context.Context, string) (*file.File, error)
	String() string
}
