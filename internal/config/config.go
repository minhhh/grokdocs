package config

import (
	"io"
	"os"

	"github.com/minhhh/grokdocs/internal/util"
	"gopkg.in/yaml.v3"
)

// CollectionConfig defines configuration for a single ingest collection.
type CollectionConfig struct {
	Path    string            `yaml:"path,omitempty"`
	Parsers map[string]string `yaml:"parsers,omitempty"`
	Files   []string          `yaml:"files,omitempty"`
	Include []string          `yaml:"include,omitempty"`
	Exclude []string          `yaml:"exclude,omitempty"`
}

// Config represents the application configuration.
type Config struct {
	Collections map[string]CollectionConfig `yaml:"collections"`
}

const (
	DefaultCollectionName = "default"
	DefaultCollectionPath = "."
)

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Collections: map[string]CollectionConfig{
			DefaultCollectionName: {
				Path: DefaultCollectionPath,
			},
		},
	}
}

// LoadConfig loads configuration from a reader.
func LoadConfig(reader io.Reader) (*Config, error) {
	var cfg Config
	dec := yaml.NewDecoder(reader)
	if err := dec.Decode(&cfg); err != nil {
		return nil, err
	}
	for name, collection := range cfg.Collections {
		if collection.Path == "" {
			collection.Path = DefaultCollectionPath
		}
		if name == DefaultCollectionName && collection.Path != "" {
			util.Logger.Warn().Str("path", collection.Path).Msg("ignoring path for default collection; Remove the path field")
			collection.Path = DefaultCollectionPath
		}
		cfg.Collections[name] = collection
	}
	return &cfg, nil
}

// SaveConfig writes configuration to a writer.
func (c *Config) SaveConfig(writer io.Writer) error {
	for name, collection := range c.Collections {
		if name == DefaultCollectionName {
			collection.Path = ""
		}
		if len(collection.Parsers) == 0 {
			collection.Parsers = nil
		}
		if len(collection.Files) == 0 {
			collection.Files = nil
		}
		if len(collection.Include) == 0 {
			collection.Include = nil
		}
		if len(collection.Exclude) == 0 {
			collection.Exclude = nil
		}
		c.Collections[name] = collection
	}
	enc := yaml.NewEncoder(writer)
	enc.SetIndent(2)
	return enc.Encode(c)
}

// LoadFromFile loads configuration from a YAML file.
func LoadFromFile(filePath string) (*Config, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return LoadConfig(file)
}

// SaveToFile saves configuration to a YAML file.
func (c *Config) SaveToFile(filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	return c.SaveConfig(file)
}
