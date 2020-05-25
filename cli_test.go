package main

import (
	"bytes"
	"fmt"
	"github.com/tkellen/memorybox/pkg/file"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tempFile(t *testing.T, content string) string {
	tempFile, tempFileErr := ioutil.TempFile("", "*")
	if tempFileErr != nil {
		t.Fatalf("test setup: %s", tempFileErr)
	}
	_, tempFileWriteErr := tempFile.WriteString(content)
	if tempFileWriteErr != nil {
		t.Fatalf("test setup: %s", tempFileWriteErr)
	}
	tempFile.Close()
	return tempFile.Name()
}

type testFiles struct {
	storePath           string
	configPath          string
	configFileHash      string
	goodIndexUpdateFile string
	badIndexUpdateFile  string
}

func testSetup(t *testing.T) testFiles {
	storePath, storePathErr := ioutil.TempDir("", "*")
	if storePathErr != nil {
		t.Fatalf("test setup: %s", storePathErr)
	}
	config := fmt.Sprintf(`targets:
  test:
    backend: localDisk
    path: %[1]s
  alternate:
    backend: localDisk
    path: %[1]s`, filepath.Join(storePath, "first"), filepath.Join(storePath, "second"))
	hash, _, _ := file.Sha256(bytes.NewBuffer([]byte(config)))
	configFile := tempFile(t, config)
	return testFiles{
		storePath:           storePath,
		configPath:          configFile,
		configFileHash:      hash,
		goodIndexUpdateFile: tempFile(t, fmt.Sprintf("{\"memorybox\":{\"file\":\"%[1]s\"}}\n{\"memorybox\":{\"file\":\"%[1]s\"}}\n", hash)),
		badIndexUpdateFile:  tempFile(t, fmt.Sprintf("{\"memorybox\":{\"file\":\"%[1]s\"}}\n{\"memorybox\":{\"file\":\"missing\"}\n{\"memorybox\":{}}\n", hash)),
	}
}

func TestRunner(t *testing.T) {
	table := map[int][]string{
		0: {
			"-d -c {{configPath}} -t test hash {{tempFile}}",
			"-d -c {{configPath}} -t test version",
			"-d -c {{configPath}} -t test put {{tempFile}}",
			"-d -c {{configPath}} -t test put {{tempFile}} && -d -c {{configPath}} -t test get {{hash}}",
			"-d -c {{configPath}} -t test put {{tempFile}} && -d -c {{configPath}} -t test meta {{hash}}",
			"-d -c {{configPath}} -t test put {{tempFile}} && -d -c {{configPath}} -t test meta {{hash}} set key value",
			"-d -c {{configPath}} -t test put {{tempFile}} && -d -c {{configPath}} -t test meta {{hash}} delete key value",
			"-d -c {{configPath}} -t test index",
			"-d -c {{configPath}} -t test put {{tempFile}} && -d -c {{configPath}} -t test index update {{goodIndexUpdateFile}}",
			"-d -c {{configPath}} -t test put {{tempFile}} && -d -c {{configPath}} -t test delete {{hash}}",
			"-d -c {{configPath}} -t test put {{tempFile}} && -d -c {{configPath}} sync metafiles test alternate",
			"-d -c {{configPath}} -t test put {{tempFile}} && -d -c {{configPath}} sync datafiles test alternate",
			"-d -c {{configPath}} -t test put {{tempFile}} && -d -c {{configPath}} sync all test alternate",
			"-d -c {{configPath}} -t test import test testdata/good-import-file",
			"-d -c testdata/config -t valid check pairing",
			"-d -c testdata/config -t valid check metafiles",
			"-d -c testdata/config -t valid check datafiles",
			"-d -c testdata/config diff valid valid",
			"-d -c {{configPath}} lambda create",
			"-d -c {{configPath}} lambda delete",
		},
		1: {
			"",
			"-d -c testdata/config help",
			"-d -c testdata/config -badflag",
			"-d -c testdata/config -t missingTarget index",
			"-d -c testdata/config -t invalid index",
			"-d -c testdata/config -t valid unknown",
			"-d -c testdata/config -t valid put",
			"-d -c testdata/config -t valid get",
			"-d -c testdata/config -t valid meta",
			"-d -c testdata/config -t valid put missing",
			"-d -c testdata/config -t valid get missing",
			"-d -c testdata/config -t valid delete missing",
			"-d -c testdata/config -t valid meta missing",
			"-d -c /root/cant/write/here/path version",
			"-d -c {{configPath}} -t test put {{tempFile}} && -d -c {{configPath}} -t test index update {{badIndexUpdateFile}}",
			"-d -c testdata/config -t object index",
			"-d -c testdata/config -t valid import test testdata/bad-import-file",
			"-d -c testdata/config -t datafile-pair-missing check pairing",
			"-d -c testdata/config -t valid check pairing",
			"-d -c testdata/config -t datafile-corrupted check datafiles",
			"-d -c testdata/config -t metafile-corrupted check metafiles",
			"-d -c testdata/config diff valid valid-alternate",
		},
	}
	for expectedCode, commands := range table {
		expectedCode := expectedCode
		for _, command := range commands {
			command := command
			t.Run(command, func(t *testing.T) {
				files := testSetup(t)
				defer os.RemoveAll(files.storePath)
				defer os.Remove(files.configPath)
				defer os.Remove(files.goodIndexUpdateFile)
				defer os.Remove(files.badIndexUpdateFile)
				commands := strings.Split(command, " && ")
				for index, cmd := range commands {
					cmd = strings.Replace(cmd, "{{configPath}}", files.configPath, -1)
					cmd = strings.Replace(cmd, "{{tempFile}}", files.configPath, -1)
					cmd = strings.Replace(cmd, "{{hash}}", files.configFileHash, -1)
					cmd = strings.Replace(cmd, "{{goodIndexUpdateFile}}", files.goodIndexUpdateFile, -1)
					cmd = strings.Replace(cmd, "{{badIndexUpdateFile}}", files.badIndexUpdateFile, -1)
					cmd = "memorybox " + cmd
					stdout := bytes.NewBuffer([]byte{})
					stderr := bytes.NewBuffer([]byte{})
					actualCode := Run(strings.Fields(cmd), stdout, stderr)
					// for commands that should exit non-zero, only check exit status of last command
					if actualCode != expectedCode && (expectedCode == 0 || (expectedCode != 0 && index == len(commands))) {
						t.Fatalf("%s exited with code %d, expected code %d\nSTDERR:\n%s\nSTDOUT:\n%s\n", cmd, actualCode, expectedCode, stderr, stdout)
					}
				}
			})
		}
	}
}
