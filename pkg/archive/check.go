package archive

import (
	"context"
	"encoding/hex"
	"fmt"
	hash "github.com/minio/sha256-simd"
	"github.com/tkellen/memorybox/pkg/file"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"io/ioutil"
	"strings"
)

const checkFmt = "%-12s%-8s%-13s%s"

type CheckOutput struct {
	Items   []CheckItem
	Details []string
}

func (co CheckOutput) String() string {
	output := []string{
		fmt.Sprintf(checkFmt, "TYPE", "COUNT", "SIGNATURE", "SOURCE"),
	}
	for _, item := range co.Items {
		output = append(output, item.String())
	}
	for _, line := range co.Details {
		if line != "" {
			output = append(output, line)
		}
	}
	return strings.Join(output, "\n")
}

type CheckItem struct {
	Name      string
	Count     int
	Signature string
	Source    string
}

func (ci CheckItem) String() string {
	return fmt.Sprintf(checkFmt, ci.Name, fmt.Sprintf("%d", ci.Count), ci.Signature[:10], ci.Source)
}

func Check(ctx context.Context, store Store, concurrency int, mode string) (*CheckOutput, error) {
	var err error
	var signature string
	var details []string
	var files file.List
	if files, err = store.Search(ctx, ""); err != nil {
		return nil, err
	}
	meta := files.Meta()
	data := files.Data()
	invalid := files.Invalid()
	result := &CheckOutput{
		Items: []CheckItem{
			{"all", len(files), nameSignature(files), "file names"},
			{"datafiles", len(data), nameSignature(data), "file names"},
			{"metafiles", len(meta), nameSignature(meta), "file names"},
			{"unpaired", len(invalid), nameSignature(invalid), "file names"},
		},
	}
	if mode == "pairing" {
		result.Details = checkPairing(invalid)
		return result, nil
	}
	var filesChecked file.List
	if mode == "metafiles" {
		filesChecked = meta
		signature, details, err = checkFiles(ctx, store, concurrency, meta)
	}
	if mode == "datafiles" {
		filesChecked = data
		signature, details, err = checkFiles(ctx, store, concurrency, data)
	}
	if filesChecked == nil {
		return nil, fmt.Errorf("unknown check mode %s", mode)
	}
	if err != nil {
		return nil, err
	}
	result.Details = details
	result.Items = append(result.Items, CheckItem{mode, len(filesChecked), signature, "file content"})
	return result, nil
}

func checkPairing(files file.List) []string {
	var errs []string
	invalid := files.Invalid()
	for _, item := range invalid.Data() {
		name := item.Name
		pair := file.MetaNameFrom(name)
		errs = append(errs, fmt.Sprintf("%s missing %s", name, pair))
	}
	for _, item := range invalid.Meta() {
		name := item.Name
		pair := file.DataNameFrom(name)
		errs = append(errs, fmt.Sprintf("%s missing %s", name, pair))
	}
	return errs
}

func checkFiles(ctx context.Context, store Store, concurrency int, files file.List) (signature string, details []string, err error) {
	signatures := make([]string, len(files))
	details = make([]string, len(files))
	eg, egCtx := errgroup.WithContext(ctx)
	sem := semaphore.NewWeighted(int64(concurrency))
	eg.Go(func() error {
		for index, name := range files.Names() {
			if err := sem.Acquire(egCtx, 1); err != nil {
				return err
			}
			index, name := index, name
			eg.Go(func() error {
				defer sem.Release(1)
				var f *file.File
				var err error
				if f, err = store.Get(egCtx, name); err != nil {
					return err
				}
				defer f.Close()
				if file.IsMetaFileName(name) {
					signatures[index], details[index], err = checkMeta(f)
				} else {
					signatures[index], details[index], err = checkData(f)
				}
				if err != nil {
					return err
				}
				return nil
			})
		}
		return nil
	})
	if err := eg.Wait(); err != nil {
		return "", nil, err
	}
	digest := hash.Sum256([]byte(strings.Join(signatures, "")))
	return hex.EncodeToString(digest[:]), details, nil
}

func checkMeta(f *file.File) (signature string, detail string, err error) {
	meta, readErr := ioutil.ReadAll(f)
	if readErr != nil {
		return "", "", readErr
	}
	digest := hash.Sum256(meta)
	if file.DataNameFrom(f.Name) != file.Meta(meta).DataFileName() {
		detail = fmt.Sprintf("%s: %s key conflicts with filename", f.Name, file.MetaKeyImportSource)
	}
	if err := file.ValidateMeta(meta); err != nil {
		detail = fmt.Sprintf("%s: %s", f.Name, err)
	}
	return hex.EncodeToString(digest[:]), detail, nil
}

func checkData(f *file.File) (signature string, detail string, err error) {
	digest, _, hashErr := file.Sha256(f)
	if hashErr != nil {
		return "", "", hashErr
	}
	f.Close()
	if f.Name != digest {
		detail = fmt.Sprintf("%s should be named %s, possible data corruption", f.Name, digest)
	}
	return digest, detail, nil
}

func nameSignature(input file.List) string {
	digest := hash.Sum256([]byte(strings.Join(input.Names(), "")))
	return hex.EncodeToString(digest[:])
}
