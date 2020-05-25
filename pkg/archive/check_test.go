package archive_test

import (
	"context"
	"github.com/google/go-cmp/cmp"
	"github.com/tkellen/memorybox/pkg/archive"
	"github.com/tkellen/memorybox/pkg/localdiskstore"
	"testing"
)

func TestCheckOutput_String(t *testing.T) {
	input := archive.CheckOutput{
		Items: []archive.CheckItem{
			{Name: "all", Count: 2, Signature: "6c40786bb260c4f38bb7dc9611c022c12e72b9c879fe2c5a7a80db1fc2fe12ef", Source: "file names"},
			{Name: "datafiles", Count: 1, Signature: "4544b50389f946f441cb7e3c107389c5f6d0f07344e748124b4541f55fc17684", Source: "file names"},
			{Name: "metafiles", Count: 1, Signature: "5c97d1b327716400029b7eb796584a7f8bf2fae9686dc11d77b2268a6d455a2c", Source: "file names"},
			{Name: "unpaired", Count: 0, Signature: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", Source: "file names"},
		},
		Details: []string{"foo", "bar", "baz"},
	}
	actual := input.String()
	expected := `TYPE        COUNT   SIGNATURE    SOURCE
all         2       6c40786bb2   file names
datafiles   1       4544b50389   file names
metafiles   1       5c97d1b327   file names
unpaired    0       e3b0c44298   file names
foo
bar
baz`
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Fatal(diff)
	}
}

func TestCheck(t *testing.T) {
	type testCase struct {
		store       *localdiskstore.Store
		expectedErr bool
		expected    *archive.CheckOutput
		mode        string
	}
	table := map[string]testCase{
		"pairing clean": {
			mode:        "pairing",
			store:       localdiskstore.New("../../testdata/valid"),
			expectedErr: false,
			expected: &archive.CheckOutput{Items: []archive.CheckItem{
				{Name: "all", Count: 2, Signature: "6c40786bb260c4f38bb7dc9611c022c12e72b9c879fe2c5a7a80db1fc2fe12ef", Source: "file names"},
				{Name: "datafiles", Count: 1, Signature: "4544b50389f946f441cb7e3c107389c5f6d0f07344e748124b4541f55fc17684", Source: "file names"},
				{Name: "metafiles", Count: 1, Signature: "5c97d1b327716400029b7eb796584a7f8bf2fae9686dc11d77b2268a6d455a2c", Source: "file names"},
				{Name: "unpaired", Count: 0, Signature: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", Source: "file names"},
			}, Details: []string(nil)},
		},
		"pairing metafile missing datafile": {
			mode:        "pairing",
			store:       localdiskstore.New("../../testdata/metafile-pair-missing"),
			expectedErr: false,
			expected: &archive.CheckOutput{Items: []archive.CheckItem{
				{Name: "all", Count: 1, Signature: "4544b50389f946f441cb7e3c107389c5f6d0f07344e748124b4541f55fc17684", Source: "file names"},
				{Name: "datafiles", Count: 1, Signature: "4544b50389f946f441cb7e3c107389c5f6d0f07344e748124b4541f55fc17684", Source: "file names"},
				{Name: "metafiles", Count: 0, Signature: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", Source: "file names"},
				{Name: "unpaired", Count: 1, Signature: "4544b50389f946f441cb7e3c107389c5f6d0f07344e748124b4541f55fc17684", Source: "file names"},
			},
				Details: []string{"b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256 missing memorybox-meta-b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256"}},
		},
		"pairing datafile missing metafile": {
			mode:        "pairing",
			store:       localdiskstore.New("../../testdata/datafile-pair-missing"),
			expectedErr: false,
			expected: &archive.CheckOutput{
				Items: []archive.CheckItem{
					{Name: "all", Count: 1, Signature: "5c97d1b327716400029b7eb796584a7f8bf2fae9686dc11d77b2268a6d455a2c", Source: "file names"},
					{Name: "datafiles", Count: 0, Signature: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", Source: "file names"},
					{Name: "metafiles", Count: 1, Signature: "5c97d1b327716400029b7eb796584a7f8bf2fae9686dc11d77b2268a6d455a2c", Source: "file names"},
					{Name: "unpaired", Count: 1, Signature: "5c97d1b327716400029b7eb796584a7f8bf2fae9686dc11d77b2268a6d455a2c", Source: "file names"},
				},
				Details: []string{"memorybox-meta-b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256 missing b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256"},
			},
		},
		"metafile corruption": {
			mode:        "metafiles",
			store:       localdiskstore.New("../../testdata/metafile-corrupted"),
			expectedErr: false,
			expected: &archive.CheckOutput{
				Items: []archive.CheckItem{
					{Name: "all", Count: 2, Signature: "6c40786bb260c4f38bb7dc9611c022c12e72b9c879fe2c5a7a80db1fc2fe12ef", Source: "file names"},
					{Name: "datafiles", Count: 1, Signature: "4544b50389f946f441cb7e3c107389c5f6d0f07344e748124b4541f55fc17684", Source: "file names"},
					{Name: "metafiles", Count: 1, Signature: "5c97d1b327716400029b7eb796584a7f8bf2fae9686dc11d77b2268a6d455a2c", Source: "file names"},
					{Name: "unpaired", Count: 0, Signature: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", Source: "file names"},
					{Name: "metafiles", Count: 1, Signature: "feb989f1d3f583caef914d19de36b2fb801f2628d7ac6c3afee97d17cd97c901", Source: "file content"},
				},
				Details: []string{"memorybox-meta-b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256: not json encoded"},
			},
		},
		"metafile clean": {
			mode:        "metafiles",
			store:       localdiskstore.New("../../testdata/valid"),
			expectedErr: false,
			expected: &archive.CheckOutput{
				Items: []archive.CheckItem{
					{Name: "all", Count: 2, Signature: "6c40786bb260c4f38bb7dc9611c022c12e72b9c879fe2c5a7a80db1fc2fe12ef", Source: "file names"},
					{Name: "datafiles", Count: 1, Signature: "4544b50389f946f441cb7e3c107389c5f6d0f07344e748124b4541f55fc17684", Source: "file names"},
					{Name: "metafiles", Count: 1, Signature: "5c97d1b327716400029b7eb796584a7f8bf2fae9686dc11d77b2268a6d455a2c", Source: "file names"},
					{Name: "unpaired", Count: 0, Signature: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", Source: "file names"},
					{Name: "metafiles", Count: 1, Signature: "9a1a3779d2abc94983aff812019a7e0b474e7b95024860362dcbcbe22c0f9660", Source: "file content"},
				},
				Details: []string{""},
			},
		},
		"datafile clean": {
			mode:        "datafiles",
			store:       localdiskstore.New("../../testdata/valid"),
			expectedErr: false,
			expected: &archive.CheckOutput{
				Items: []archive.CheckItem{
					{Name: "all", Count: 2, Signature: "6c40786bb260c4f38bb7dc9611c022c12e72b9c879fe2c5a7a80db1fc2fe12ef", Source: "file names"},
					{Name: "datafiles", Count: 1, Signature: "4544b50389f946f441cb7e3c107389c5f6d0f07344e748124b4541f55fc17684", Source: "file names"},
					{Name: "metafiles", Count: 1, Signature: "5c97d1b327716400029b7eb796584a7f8bf2fae9686dc11d77b2268a6d455a2c", Source: "file names"},
					{Name: "unpaired", Count: 0, Signature: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", Source: "file names"},
					{Name: "datafiles", Count: 1, Signature: "4544b50389f946f441cb7e3c107389c5f6d0f07344e748124b4541f55fc17684", Source: "file content"},
				},
				Details: []string{""},
			},
		},
	}
	for name, test := range table {
		test := test
		t.Run(name, func(t *testing.T) {
			actual, err := archive.Check(context.Background(), test.store, 10, test.mode)
			if test.expectedErr && err == nil {
				t.Fatalf("expected error, got none")
			}
			if !test.expectedErr && err != nil {
				t.Fatalf("expected no error, got %s", err)
			}
			if actual != nil {
				if diff := cmp.Diff(test.expected, actual); diff != "" {
					t.Fatal(diff)
				}
			}
		})
	}
}
