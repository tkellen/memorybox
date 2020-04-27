package commands

import (
	"fmt"
	"github.com/tkellen/memorybox/internal/configfile"
)

// ConfigShow shows the full contents of a configfile.
func ConfigShow(configFile *configfile.ConfigFile, logger func(format string, v ...interface{})) error {
	logger(fmt.Sprintf("%s", configFile))
	return nil
}

// ConfigSet configures a key/value pair for a defined target.
func ConfigSet(configFile *configfile.ConfigFile, target string, key string, value string) error {
	configFile.Target(target).Set(key, value)
	return nil
}

// ConfigDelete removes a configuration key for a target.
func ConfigDelete(configFile *configfile.ConfigFile, target string, key string) error {
	if key != "" {
		configFile.Target(target).Delete(key)
	} else {
		configFile.Delete(target)
	}
	return nil
}
