package main

import (
	"bytes"
	"fmt"
	"github.com/tkellen/memorybox/pkg/archive"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

type testFiles struct {
	storePath      string
	configPath     string
	configFileHash string
	importFilePath string
}

func testSetup(t *testing.T) testFiles {
	storePath, storePathErr := ioutil.TempDir("", "*")
	if storePathErr != nil {
		t.Fatalf("test setup: %s", storePathErr)
	}
	config := []byte(fmt.Sprintf(`targets:
  test:
    type: localDisk
    path: %s
  invalid:
    type: whatever`, storePath))
	hash, _, _ := archive.Sha256(bytes.NewBuffer(config))
	configFile, configFileErr := ioutil.TempFile("", "*")
	if configFileErr != nil {
		t.Fatalf("test setup: %s", configFileErr)
	}
	_, configFileWriteErr := configFile.Write(config)
	if configFileWriteErr != nil {
		t.Fatalf("test setup: %s", configFileWriteErr)
	}
	configFile.Close()
	importFile, importFileErr := ioutil.TempFile("", "*")
	if importFileErr != nil {
		t.Fatalf("test setup: %s", configFileErr)
	}
	_, importFileWriteErr := importFile.WriteString(fmt.Sprintf("%s\t{\"test\":\"meta\"}", configFile.Name()))
	if importFileWriteErr != nil {
		t.Fatalf("test setup: %s", importFileWriteErr)
	}
	configFile.Close()
	return testFiles{
		storePath:      storePath,
		configPath:     configFile.Name(),
		configFileHash: hash,
		importFilePath: importFile.Name(),
	}
}

func TestRunner(t *testing.T) {
	table := map[string]int{
		"":                           1,
		"-c {{configPath}} help":     1,
		"-c {{configPath}} -badflag": 1,
		"-c {{configPath}} -t missingTarget index":                                                               1,
		"-c {{configPath}} -t invalid index":                                                                     1,
		"-c {{configPath}} -t test unknown":                                                                      1,
		"-c {{configPath}} -t test put":                                                                          1,
		"-c {{configPath}} -t test get":                                                                          1,
		"-c {{configPath}} -t test meta":                                                                         1,
		"-c {{configPath}} -t test put missing":                                                                  1,
		"-c {{configPath}} -t test get missing":                                                                  1,
		"-c {{configPath}} -t test meta missing":                                                                 1,
		"-c /root/cant/write/here/path version":                                                                  1,
		"-c {{configPath}} -d version":                                                                           0,
		"-c {{configPath}} -t test version":                                                                      0,
		"-c {{configPath}} -t test put {{tempFile}}":                                                             0,
		"-c {{configPath}} -t test put {{tempFile}} && -c {{configPath}} -t test get {{hash}}":                   0,
		"-c {{configPath}} -t test put {{tempFile}} && -c {{configPath}} -t test meta {{hash}}":                  0,
		"-c {{configPath}} -t test put {{tempFile}} && -c {{configPath}} -t test meta {{hash}} set key value":    0,
		"-c {{configPath}} -t test put {{tempFile}} && -c {{configPath}} -t test meta {{hash}} delete key value": 0,
		"-c {{configPath}} -t test index":                                                                        0,
		"-c {{configPath}} -t test index rehash":                                                                 0,
		"-c {{configPath}} -t test import {{importFile}}":                                                        0,
	}
	for command, expectedCode := range table {
		expectedCode := expectedCode
		t.Run(command, func(t *testing.T) {
			files := testSetup(t)
			defer os.RemoveAll(files.storePath)
			defer os.Remove(files.configPath)
			defer os.Remove(files.importFilePath)
			commands := strings.Split(command, " && ")
			for _, cmd := range commands {
				cmd = strings.Replace(cmd, "{{tempFile}}", files.configPath, -1)
				cmd = strings.Replace(cmd, "{{hash}}", files.configFileHash[0:6], -1)
				cmd = strings.Replace(cmd, "{{importFile}}", files.importFilePath, -1)
				cmd = strings.Replace(cmd, "{{configPath}}", files.configPath, -1)
				cmd = "memorybox " + cmd
				stdout := bytes.NewBuffer([]byte{})
				stderr := bytes.NewBuffer([]byte{})
				actualCode, err := Run(strings.Fields(cmd), stderr, stdout)
				if actualCode != expectedCode {
					t.Fatalf("%s errored %s with code %d, expected code %d\nSTDERR:\n%s\nSTDOUT:\n%s\n", cmd, err, actualCode, expectedCode, stderr, stdout)
				}
			}
		})
	}
}
