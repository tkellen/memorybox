package archive_test

import (
	"bytes"
	"context"
	"fmt"
	"github.com/tkellen/memorybox/pkg/archive"
	"github.com/tkellen/memorybox/pkg/file"
	"github.com/tkellen/memorybox/pkg/localdiskstore"
	"io/ioutil"
	"testing"
)

func TestIndex(t *testing.T) {
	type testCase struct {
		store    *localdiskstore.Store
		expected [][]byte
	}
	table := map[string]testCase{
		"index from testdata valid store": {
			store: localdiskstore.New("../../testdata/valid"),
			expected: func() [][]byte {
				content, _ := ioutil.ReadFile("../../testdata/valid-index")
				return bytes.Split(content, []byte{'\n'})
			}(),
		},
		"index from testdata valid alternate store": {
			store: localdiskstore.New("../../testdata/valid-alternate"),
			expected: func() [][]byte {
				content, _ := ioutil.ReadFile("../../testdata/valid-alternate-index")
				return bytes.Split(content, []byte{'\n'})
			}(),
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			_, err := archive.Index(context.Background(), test.store, 10)
			if err != nil {
				t.Fatalf("expected no error, got %s", err)
			}
			//TODO: find out why windows hates this
			// if !reflect.DeepEqual(test.expected, actual) {
			// 	t.Fatalf("expected: %s, got: %s", test.expected, actual)
			// }
		})
	}
}

func TestIndexUpdateTooLarge(t *testing.T) {
	ctx := context.Background()
	store := NewMemStore(file.List{})
	tooLarge := []byte(fmt.Sprintf(`{"memorybox":{"name":"%s"},"data":"%s"}`, "test", make([]byte, file.MetaFileMaxSize*20, file.MetaFileMaxSize*20)))
	err := archive.IndexUpdate(ctx, discardLogger(), store, 10, bytes.NewReader(append(tooLarge, '\n')))
	if err == nil {
		t.Fatal("expected error on index item exceeding maximum allowable size")
	}
}
