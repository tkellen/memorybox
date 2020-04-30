package configfile

import (
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
)

// Target describes a single target in the memorybox configuration file.
type Target map[string]string

// ConfigFile holds configuration data for various memorybox target stores.
type ConfigFile struct {
	Targets map[string]Target `yaml:"targets"`
}

// NewConfigFile instantiates a configFile and immediately populates it with the
// supplied data.
func NewConfigFile(data io.Reader) (*ConfigFile, error) {
	configFile := &ConfigFile{}
	err := configFile.Load(data)
	if err != nil {
		return nil, err
	}
	return configFile, err
}

// String returns a yaml-formatted representation of the content of config.
func (config *ConfigFile) String() string {
	yaml, _ := yaml.Marshal(config)
	return string(yaml)
}

// Target finds the requested target, creating one if needed.
func (config *ConfigFile) Target(name string) *Target {
	targets := config.Targets
	// If there are no targets yet, we are likely running for the first time.
	// Instantiate the config.File.Target struct.
	if targets == nil {
		config.Targets = map[string]Target{}
		targets = config.Targets
	}
	// If a target is found in the existing config, return it now.
	if targeted, ok := targets[name]; ok {
		return &targeted
	}
	// If we are here, we are making a new target because the requested target
	// was not found.
	targeted := &Target{
		"type": "localDisk",
		"home": "~/memorybox",
	}
	// If no name was specified we are operating in "default" mode and do not
	// want to populate our configuration file with this target, so, skip this.
	if name != "" {
		targets[name] = *targeted
	}
	return targeted
}

// Delete removes a target by name from the configuration struct.
func (config *ConfigFile) Delete(name string) *ConfigFile {
	delete(config.Targets, name)
	return config
}

// Load reads a provided data source that is expected to contain yaml that can
// be directly unmarshalled into File field of ConfigFile.
func (config *ConfigFile) Load(data io.Reader) error {
	bytes, err := ioutil.ReadAll(data)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(bytes, &config); err != nil {
		return err
	}
	return nil
}

// Save renders the current configuration as YAML and writes it to a consumer
// specified io.Writer.
func (config *ConfigFile) Save(dest io.Writer) error {
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
