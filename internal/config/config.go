package config

import (
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// CollectionConfig defines configuration for a single ingest collection.
type CollectionConfig struct {
	Path    string            `yaml:"path"`
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
func LoadConfig(r io.Reader) (*Config, error) {
	var cfg Config
	dec := yaml.NewDecoder(r)
	if err := dec.Decode(&cfg); err != nil {
		return nil, err
	}
	for name, col := range cfg.Collections {
		if col.Path == "" {
			col.Path = DefaultCollectionPath
			cfg.Collections[name] = col
		}
	}
	return &cfg, nil
}

// SaveConfig writes configuration to a writer.
func (c *Config) SaveConfig(w io.Writer) error {
	for name, col := range c.Collections {
		if len(col.Parsers) == 0 {
			col.Parsers = nil
		}
		if len(col.Files) == 0 {
			col.Files = nil
		}
		if len(col.Include) == 0 {
			col.Include = nil
		}
		if len(col.Exclude) == 0 {
			col.Exclude = nil
		}
		c.Collections[name] = col
	}
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	return enc.Encode(c)
}

// LoadFromFile loads configuration from a YAML file.
func LoadFromFile(filePath string) (*Config, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return LoadConfig(f)
}

// SaveToFile saves configuration to a YAML file.
func (c *Config) SaveToFile(filePath string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	return c.SaveConfig(f)
}
