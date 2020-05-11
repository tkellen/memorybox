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
	storePath           string
	configPath          string
	configFileHash      string
	goodImportFile      string
	goodIndexUpdateFile string
	badImportFile       string
	badIndexUpdateFile  string
}

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

func testSetup(t *testing.T) testFiles {
	storePath, storePathErr := ioutil.TempDir("", "*")
	if storePathErr != nil {
		t.Fatalf("test setup: %s", storePathErr)
	}
	config := fmt.Sprintf(`targets:
  test:
    type: localDisk
    path: %s
  invalid:
    type: whatever`, storePath)
	hash, _, _ := archive.Sha256(bytes.NewBuffer([]byte(config)))
	configFile := tempFile(t, config)
	return testFiles{
		storePath:           storePath,
		configPath:          configFile,
		configFileHash:      hash,
		goodImportFile:      tempFile(t, fmt.Sprintf("%s {\"test\":\"meta\"}\n", configFile)),
		goodIndexUpdateFile: tempFile(t, fmt.Sprintf("{\"memorybox\":{\"file\":\"%[1]s\"}}\n{\"memorybox\":{\"file\":\"%[1]s\"}}\n", hash)),
		badImportFile:       tempFile(t, "239487621384792,,,\n\n12312346asfkjJASKF*231 \n "),
		badIndexUpdateFile:  tempFile(t, fmt.Sprintf("{\"memorybox\":{\"file\":\"%[1]s\"}}\n{\"memorybox\":{\"file\":\"missing\"}\n{\"memorybox\":{}}\n", hash)),
	}
}

func TestRunner(t *testing.T) {
	table := map[int][]string{
		0: {
			"-c {{configPath}} -d version",
			"-c {{configPath}} -t test version",
			"-c {{configPath}} -t test hash {{tempFile}}",
			"-c {{configPath}} -t test put {{tempFile}}",
			"-c {{configPath}} -t test put {{tempFile}} && -c {{configPath}} -t test get {{hash}}",
			"-c {{configPath}} -t test put {{tempFile}} && -c {{configPath}} -t test meta {{hash}}",
			"-c {{configPath}} -t test put {{tempFile}} && -c {{configPath}} -t test meta {{hash}} set key value",
			"-c {{configPath}} -t test put {{tempFile}} && -c {{configPath}} -t test meta {{hash}} delete key value",
			"-c {{configPath}} -t test index",
			"-c {{configPath}} -t test put {{tempFile}} && -c {{configPath}} -t test index rehash",
			"-c {{configPath}} -t test put {{tempFile}} && -c {{configPath}} -t test index update {{goodIndexUpdateFile}}",
			"-c {{configPath}} -t test put {{tempFile}} && -c {{configPath}} -t test delete {{hash}}",
			"-c {{configPath}} -t test import {{goodImportFile}}",
		},
		1: {
			"",
			"-c {{configPath}} help",
			"-c {{configPath}} -badflag",
			"-c {{configPath}} -t missingTarget index",
			"-c {{configPath}} -t invalid index",
			"-c {{configPath}} -t test unknown",
			"-c {{configPath}} -t test put",
			"-c {{configPath}} -t test get",
			"-c {{configPath}} -t test meta",
			"-c {{configPath}} -t test put missing",
			"-c {{configPath}} -t test get missing",
			"-c {{configPath}} -t test delete missing",
			"-c {{configPath}} -t test meta missing",
			"-c /root/cant/write/here/path version",
			"-c {{configPath}} -t test put {{tempFile}} && -c {{configPath}} -t test index update {{badIndexUpdateFile}}",
			"-c {{configPath}} -t test import {{badImportFile}}",
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
				defer os.Remove(files.goodImportFile)
				defer os.Remove(files.goodIndexUpdateFile)
				defer os.Remove(files.badImportFile)
				defer os.Remove(files.badIndexUpdateFile)
				commands := strings.Split(command, " && ")
				for index, cmd := range commands {
					cmd = strings.Replace(cmd, "{{configPath}}", files.configPath, -1)
					cmd = strings.Replace(cmd, "{{tempFile}}", files.configPath, -1)
					cmd = strings.Replace(cmd, "{{hash}}", files.configFileHash, -1)
					cmd = strings.Replace(cmd, "{{goodImportFile}}", files.goodImportFile, -1)
					cmd = strings.Replace(cmd, "{{goodIndexUpdateFile}}", files.goodIndexUpdateFile, -1)
					cmd = strings.Replace(cmd, "{{badImportFile}}", files.badImportFile, -1)
					cmd = strings.Replace(cmd, "{{badIndexUpdateFile}}", files.badIndexUpdateFile, -1)
					cmd = "memorybox " + cmd
					stdout := bytes.NewBuffer([]byte{})
					stderr := bytes.NewBuffer([]byte{})
					actualCode := Run(strings.Fields(cmd), stderr, stdout)
					// for commands that should exit non-zero, only check exit status of last command
					if actualCode != expectedCode && (expectedCode == 0 || (expectedCode != 0 && index == len(commands))) {
						t.Fatalf("%s exited with code %d, expected code %d\nSTDERR:\n%s\nSTDOUT:\n%s\n", cmd, actualCode, expectedCode, stderr, stdout)
					}
				}
			})
		}
	}
}
