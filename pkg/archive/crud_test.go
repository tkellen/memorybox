package archive_test

import (
	"context"
	"github.com/mattetti/filebuffer"
	"github.com/tkellen/memorybox/pkg/archive"
	"github.com/tkellen/memorybox/pkg/file"
	"io/ioutil"
	"os"
	"testing"
	"time"
)

func TestPut(t *testing.T) {
	ctx := context.Background()
	testStore := NewMemStore([]*file.File{})
	fromExpected, hostErr := os.Hostname()
	if hostErr != nil {
		t.Fatal(hostErr)
	}
	f, err := file.NewSha256("test", filebuffer.New([]byte("test")), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := testStore.Stat(ctx, f.Name); err == nil {
		t.Fatal("store should not have datafile yet")
	}
	if _, err := testStore.Stat(ctx, file.MetaNameFrom(f.Name)); err == nil {
		t.Fatal("store should not have metafile yet")
	}
	if _, err := archive.Put(ctx, testStore, f, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := testStore.Stat(ctx, f.Name); err != nil {
		t.Fatal("expected to find datafile after put")
	}
	metaReader, getErr := testStore.Get(ctx, file.MetaNameFrom(f.Name))
	if getErr != nil {
		t.Fatal("expected to find metafile after put")
	}
	metaBytes, readErr := ioutil.ReadAll(metaReader)
	if readErr != nil {
		t.Fatal(readErr)
	}
	meta := file.Meta(metaBytes)
	fromActual := meta.Get(file.MetaKeyImportSet).(string)
	if fromExpected != fromActual {
		t.Fatalf("expected source to be %s, got %s", fromExpected, fromActual)
	}
}

func TestPutWontOverwrite(t *testing.T) {
	ctx := context.Background()
	testStore := NewMemStore([]*file.File{})
	expectedMetaValue := "key"
	f, err := file.NewSha256("test", filebuffer.New([]byte("test")), time.Now())
	f.Meta.Set("test", expectedMetaValue)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := testStore.Stat(ctx, f.Name); err == nil {
		t.Fatal("store should not have datafile yet")
	}
	if _, err := testStore.Stat(ctx, file.MetaNameFrom(f.Name)); err == nil {
		t.Fatal("store should not have metafile yet")
	}
	if _, err := archive.Put(ctx, testStore, f, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := testStore.Stat(ctx, f.Name); err != nil {
		t.Fatal("expected to find datafile after put")
	}
}

func TestDelete(t *testing.T) {
	ctx := context.Background()
	datafile, err := file.NewSha256("test", filebuffer.New([]byte("test")), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	metafile := file.NewStub(file.MetaNameFrom(datafile.Name), 0, time.Now())
	metafile.Meta = datafile.Meta
	testStore := NewMemStore(file.List{datafile, metafile})
	if _, err := testStore.Stat(ctx, datafile.Name); err != nil {
		t.Fatal("store should have datafile")
	}
	if _, err := testStore.Stat(ctx, metafile.Name); err != nil {
		t.Fatal("store should have metafile")
	}
	if err := archive.Delete(ctx, testStore, datafile.Name); err != nil {
		t.Fatal(err)
	}
	if _, err := testStore.Stat(ctx, datafile.Name); err == nil {
		t.Fatal("store should no longer have datafile")
	}
	if _, err := testStore.Stat(ctx, metafile.Name); err == nil {
		t.Fatal("store should no longer have metafile")
	}
}
