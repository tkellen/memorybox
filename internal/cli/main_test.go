package cli_test

import (
	"errors"
	"fmt"
	"github.com/tkellen/memorybox/internal/cli"
	"github.com/tkellen/memorybox/internal/simplecli"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"testing"
)

const goodConfig = `
targets:
  test:
    type: testing
    key: value
  badTarget:
    type: unknown
    key: value
`
const badConfig = "[notyaml]"

func TestRunner(t *testing.T) {
	silentLogger := log.New(ioutil.Discard, "", 0)
	// make a good / bad configuration file to use for each command
	configs := map[string]string{
		"good": goodConfig,
		"bad":  badConfig,
	}
	for name, content := range configs {
		file, err := ioutil.TempFile("", "*")
		if err != nil {
			t.Fatalf("test setup: %s", err)
		}
		file.WriteString(content)
		file.Close()
		configs[name] = file.Name()
	}
	defer os.Remove(configs["good"])
	defer os.Remove(configs["bad"])

	table := map[string]struct {
		command          string
		configPath       string
		flagVariations   []string
		configVariations map[string]string
		expectedErr      error
	}{
		"non-existent command": {
			command: "hum",
			configVariations: map[string]string{
				"good": configs["good"],
			},
			flagVariations: []string{""},
			expectedErr:    errors.New("Usage"),
		},
		"show config": {
			command:          "config",
			configVariations: configs,
			flagVariations:   []string{""},
			expectedErr:      nil,
		},
		"config set key on target": {
			command:          "config set test key someValue",
			configVariations: configs,
			flagVariations:   []string{"", "-d "},
			expectedErr:      nil,
		},
		"config delete key on target": {
			command:          "config delete test key",
			configVariations: configs,
			flagVariations:   []string{"", "-d"},
			expectedErr:      nil,
		},
		"config delete entire target": {
			command:          "config delete test",
			configVariations: configs,
			flagVariations:   []string{"", "-d"},
			expectedErr:      nil,
		},
		"put file in store": {
			command:          "put test " + configs["good"],
			configVariations: configs,
			flagVariations:   []string{"", "-d", "-c 2"},
			expectedErr:      nil,
		},
		"put file in target with bad store type configured": {
			command:          "put badTarget " + configs["good"],
			configVariations: configs,
			flagVariations:   []string{"", "-d", "-c 2"},
			expectedErr:      errors.New("unknown store type"),
		},
		"get file from store": {
			command:          "get test test",
			configVariations: configs,
			flagVariations:   []string{"", "-d"},
			expectedErr:      errors.New("0 objects"),
		},
		"get metadata on missing metadata": {
			command:          "meta test 3a",
			configVariations: configs,
			flagVariations:   []string{"", "-d"},
			expectedErr:      errors.New("0 objects"),
		},
		"set metadata on missing metadata": {
			command:          "meta test 3a set newKey someValue",
			configVariations: configs,
			flagVariations:   []string{"", "-d"},
			expectedErr:      errors.New("0 objects"),
		},
		"delete metadata on missing metadata": {
			command:          "meta test 3a delete newKey",
			configVariations: configs,
			flagVariations:   []string{"", "-d"},
			expectedErr:      errors.New("0 objects"),
		},
	}
	for name, test := range table {
		test := test
		// run command with all configuration files specified
		for configType, configPath := range test.configVariations {
			testName := fmt.Sprintf("with %s config %s", name, configType)
			expectedErr := test.expectedErr
			// on runs with a config that is "bad", expect failure on unmarshalling
			if configType == "bad" {
				expectedErr = errors.New("unmarshal")
			}
			// for every test, run variations with flags added and removed
			for _, variation := range test.flagVariations {
				args := strings.Fields(fmt.Sprintf("memorybox %s "+test.command, variation))
				t.Run(testName+" "+strings.Join(args, " "), func(t *testing.T) {
					tempDir, err := ioutil.TempDir("", "*")
					if err != nil {
						t.Fatal(err)
					}
					defer os.RemoveAll(tempDir)
					runner := cli.New(silentLogger)
					(*runner).PathConfig = configPath
					runErr := simplecli.Run(runner, args)
					if runErr != nil && expectedErr == nil {
						t.Fatalf("did not expect: %s", runErr)
					}
					if runErr != nil && expectedErr != nil && !strings.Contains(runErr.Error(), expectedErr.Error()) {
						t.Fatalf("expected error %s, got %s", expectedErr, runErr)
					}
				})
			}
		}
	}
}
