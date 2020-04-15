// Package config is responsible for providing an interface to the combination
// of a configuration file and command line options provided during execution.
package config

import (
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
)

// Target describes a single target in the memorybox configuration file.
type Target map[string]string

// File describes the structure of the memorybox configuration file.
type File struct {
	Targets map[string]Target `yaml:"targets"`
}

// Flags provides a typed interface to all supported command line arguments.
// Though this module has no awareness of it, these flags were designed to be
// populated through the use of docopts.
type Flags struct {
	Config      bool
	Show        bool
	Delete      bool
	Target      string
	Set         bool
	Key         string
	Value       string
	Put         bool
	Get         bool
	Files       []string
	Concurrency int
	Debug       bool
}

// Config combines the File and Flags structs to create a unified understanding
// of a request being made from the command line.
type Config struct {
	File
	Flags
}

// Target finds the requested target, creating one if needed.
func (config *Config) Target() *Target {
	lookup := config.Flags.Target
	targets := config.File.Targets
	// If there are no targets yet, we are likely running for the first time on
	// a new computer. Instantiate the config.File.Target struct.
	if targets == nil {
		config.File.Targets = map[string]Target{}
		targets = config.File.Targets
	}
	// If we found the target we want in our existing config, return it now.
	if targeted, ok := targets[lookup]; ok {
		return &targeted
	}
	// If we are here, we are making a new target because the requested target
	// was not found.
	targeted := &Target{
		"type": "localdisk",
		"home": "~/memorybox",
	}
	// If no lookup was specified we are operating in "default" mode and do not
	// want to populate our configuration file with this target, so, skip this.
	if lookup != "" {
		targets[lookup] = *targeted
	}
	return targeted
}

// Delete removes a target by name from the configuration struct.
func (config *Config) Delete(name string) *Config {
	delete(config.File.Targets, name)
	return config
}

// Load reads a provided data source that is expected to contain yaml that can
// be directly unmarshalled into File field of Config.
func (config *Config) Load(data io.Reader) error {
	bytes, err := ioutil.ReadAll(data)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(bytes, &config.File); err != nil {
		return err
	}
	return nil
}

// Save renders the current configuration as YAML and writes it to a consumer
// specified io.Writer.
func (config *Config) Save(dest io.Writer) error {
	yaml, _ := yaml.Marshal(config.File)
	// validate number of bytes written too?
	if _, err := dest.Write(yaml); err != nil {
		return err
	}
	return nil
}

// String returns a yaml-formatted representation of the content of File.
func (file *File) String() string {
	yaml, _ := yaml.Marshal(file)
	return string(yaml)
}

// Set assigns a configuration value to the target without consumers knowing
// where it is being assigned.
func (target *Target) Set(key string, value string) *Target {
	(*target)[key] = value
	return target
}

// Get retrieves a configuration value from a target without consumers knowing
// where it was stored.
func (target *Target) Get(key string) string {
	return (*target)[key]
}
