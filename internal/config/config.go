package config

import (
	"bytes"
	"fmt"
	"github.com/mitchellh/go-homedir"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"os"
	"path"
)

// Target describes a single target in the configuration file.
type Target map[string]string

// Config holds configuration data for various targets.
type Config struct {
	Targets map[string]Target `yaml:"targets"`
	file    *os.File
}

// New instantiates a config and immediately populates it with the
// supplied data.
func New(data io.Reader) (*Config, error) {
	cfg := &Config{
		Targets: map[string]Target{
			"default": {
				"type": "localDisk",
				"path": "~/memorybox",
			},
		},
	}
	err := cfg.Load(data)
	if err != nil {
		return nil, err
	}
	return cfg, err
}

func NewFromFile(location string) (*Config, error) {
	// Find full path to configuration file.
	fullPath, _ := homedir.Expand(location)
	// Ensure configuration directory exists.
	if err := os.MkdirAll(path.Dir(fullPath), 0755); err != nil {
		return nil, err
	}
	// Open / ensure configuration file exists.
	file, fileErr := os.OpenFile(fullPath, os.O_RDWR|os.O_CREATE, 0644)
	if fileErr != nil {
		return nil, fileErr
	}
	cfg, err := New(file)
	if err != nil {
		return nil, err
	}
	cfg.file = file
	return cfg, nil
}

func NewFromEnvOrFile(location string, envvar string) (*Config, error) {
	configEnv := os.Getenv(envvar)
	if configEnv != "" {
		return New(bytes.NewBufferString(configEnv))
	}
	return NewFromFile(location)
}

// String returns a yaml-formatted representation of the content of config.
func (config *Config) String() string {
	yaml, _ := yaml.Marshal(config)
	return string(yaml)
}

// Create inserts a new target.
func (config *Config) Create(name string, storeType string) *Config {
	targets := config.Targets
	if _, ok := targets[name]; !ok {
		config.Targets[name] = Target{
			"backend": storeType,
		}
	}
	return config
}

// Target finds the requested target, creating one if needed.
func (config *Config) Target(name string) (*Target, error) {
	targets := config.Targets
	if targeted, ok := targets[name]; ok {
		return &targeted, nil
	}
	return nil, fmt.Errorf("%s target not found", name)
}

// Delete removes a target by name from the configuration struct.
func (config *Config) Delete(name string) *Config {
	delete(config.Targets, name)
	return config
}

// Load reads a provided data source that is expected to contain yaml that can
// be directly unmarshalled into File field of Config.
func (config *Config) Load(data io.Reader) error {
	bytes, err := ioutil.ReadAll(data)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(bytes, &config); err != nil {
		return err
	}
	return nil
}

func (config *Config) Save() error {
	if config.file == nil {
		return fmt.Errorf("no underlying file found")
	}
	config.file.Seek(0, io.SeekStart)
	config.file.Truncate(0)
	defer config.file.Close()
	return config.SaveFrom(config.file)
}

// SaveFrom renders the current configuration as YAML and writes it to a
// consumer specified io.Writer.
func (config *Config) SaveFrom(dest io.Writer) error {
	yaml, _ := yaml.Marshal(config)
	// validate number of bytes written too?
	if _, err := dest.Write(yaml); err != nil {
		return err
	}
	return nil
}

// Set assigns a configuration value to the target.
func (target *Target) Set(key string, value string) *Target {
	(*target)[key] = value
	return target
}

// Delete removes a configuration value.
func (target *Target) Delete(key string) *Target {
	delete(*target, key)
	return target
}

// Get retrieves a configuration value from a target without consumers knowing
// where it was stored.
func (target *Target) Get(key string) string {
	return (*target)[key]
}
