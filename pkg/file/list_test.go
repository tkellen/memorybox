package file_test

import (
	"github.com/tkellen/memorybox/pkg/file"
	"sort"
	"testing"
)

func TestList(t *testing.T) {
	fl := file.List{
		&file.File{Name: "second"},
		&file.File{Name: "first"},
		&file.File{Name: "third"},
	}
	sort.Sort(fl)
	for index, expected := range []string{"first", "second", "third"} {
		actual := fl[index].Name
		if actual != expected {
			t.Fatalf("expected sort to have %s at %d index, got %s", expected, index, actual)
		}
	}
}
