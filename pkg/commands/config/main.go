// Package config is a high level abstraction over configfile.Config. It is
// designed to execute operations that are supported by the cli.
package config

import (
	"fmt"
	"github.com/tkellen/memorybox/pkg/configfile"
)

// Commands defines the interface needed to expose this functionality in a way
// that can be mocked for testing by consumers.
type Commands interface {
	Main(*configfile.ConfigFile) error
	Set(*configfile.ConfigFile, string, string, string) error
	Delete(*configfile.ConfigFile, string, string) error
}

// Command implements Commands as the public API of this package.
type Command struct{}

// Main shows the full contents of the configfile.
func (Command) Main(configFile *configfile.ConfigFile) error {
	return fmt.Errorf("%s", configFile)
}

// Set configures a key/value pair for a defined target.
func (Command) Set(configFile *configfile.ConfigFile, target string, key string, value string) error {
	configFile.Target(target).Set(key, value)
	return nil
}

// Delete removes a configuration key for a target.
func (Command) Delete(configFile *configfile.ConfigFile, target string, key string) error {
	if key != "" {
		configFile.Target(target).Delete(key)
	} else {
		configFile.Delete(target)
	}
	return nil
}
