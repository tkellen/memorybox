package commands_test

import (
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/tkellen/memorybox/commands"
	"github.com/tkellen/memorybox/internal/configfile"
	"testing"
)

// These test are pretty much phoning it home, but, the functionality is so
// simple it seems almost silly to test at all.
func TestConfig(t *testing.T) {
	// set up empty config
	config := &configfile.ConfigFile{}
	// confirm ConfigShow sends the string representation of the config to the
	// provided logger.
	actualOutput := ""
	expectedOutput := config.String()
	commands.ConfigShow(config, func(format string, v ...interface{}) {
		actualOutput = fmt.Sprintf(format, v...)
	})
	if diff := cmp.Diff(expectedOutput, actualOutput); diff != "" {
		t.Fatal(diff)
	}
	// confirm ConfigSet sets a value in a target on the provided config.
	target := "test"
	key := "test"
	value := "test"
	if err := commands.ConfigSet(config, target, key, value); err != nil {
		t.Fatal(err)
	}
	actualTargetKey := config.Target(target).Get(key)
	if diff := cmp.Diff(value, actualTargetKey); diff != "" {
		t.Fatal(diff)
	}
	// confirm ConfigDelete removes a value in a target on the provide config
	if err := commands.ConfigDelete(config, target, key); err != nil {
		t.Fatal(err)
	}
	keyAfterDelete := config.Target(target).Get(key)
	if diff := cmp.Diff(keyAfterDelete, ""); diff != "" {
		t.Fatal(diff)
	}
	// confirm ConfigDelete deletes a target entirely on the provided config
	if err := commands.ConfigDelete(config, target, ""); err != nil {
		t.Fatal(err)
	}
	// config should be empty of targets again
	if config.String() != "targets: {}\n" {
		t.Fatal("expected config to be empty")
	}
}
