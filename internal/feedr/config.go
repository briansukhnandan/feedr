package feedr

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	configFileName = "config.json"
	stateDirName   = ".state"
)

type Config struct {
	PollIntervalSeconds int               `json:"pollIntervalSeconds,omitempty"`
	Timezone            string            `json:"timezone,omitempty"`
	Publishers          []PublisherConfig `json:"publishers"`
}

type PublisherConfig struct {
	ID             string `json:"id"`
	Type           string `json:"type"`
	Identifier     string `json:"identifier,omitempty"`
	IdentifierEnv  string `json:"identifierEnv,omitempty"`
	AppPassword    string `json:"appPassword,omitempty"`
	AppPasswordEnv string `json:"appPasswordEnv,omitempty"`
	Service        string `json:"service,omitempty"`
}

func DataDirFromEnvironment() (string, error) {
	if dir := os.Getenv("FEEDR_HOME"); dir != "" {
		return filepath.Abs(dir)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	return filepath.Join(home, ".feedr"), nil
}

func readConfig(dir string) (Config, error) {
	file, err := os.Open(filepath.Join(dir, configFileName))
	if err != nil {
		return Config{}, fmt.Errorf("read configuration: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var config Config
	if err := decoder.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("decode configuration: %w", err)
	}
	if len(config.Publishers) == 0 {
		return Config{}, fmt.Errorf("configuration has no publishers")
	}
	if config.PollIntervalSeconds == 0 {
		config.PollIntervalSeconds = 60
	}
	if config.PollIntervalSeconds < 5 {
		return Config{}, fmt.Errorf("pollIntervalSeconds must be at least 5")
	}
	if config.Timezone == "" {
		config.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(config.Timezone); err != nil {
		return Config{}, fmt.Errorf("timezone %q is invalid: %w", config.Timezone, err)
	}

	seen := make(map[string]bool, len(config.Publishers))
	for i, publisher := range config.Publishers {
		if publisher.ID == "" || publisher.Type == "" {
			return Config{}, fmt.Errorf("publisher %d requires id and type", i)
		}
		if seen[publisher.ID] {
			return Config{}, fmt.Errorf("publisher id %q is duplicated", publisher.ID)
		}
		seen[publisher.ID] = true
		if publisher.Type != "bluesky" {
			return Config{}, fmt.Errorf("publisher %q has unsupported type %q", publisher.ID, publisher.Type)
		}
	}
	return config, nil
}

func (c Config) location() *time.Location {
	// readConfig has already validated this value.
	location, _ := time.LoadLocation(c.Timezone)
	return location
}

func (p PublisherConfig) credential(value, environment, label string) (string, error) {
	if environment != "" {
		if resolved := os.Getenv(environment); resolved != "" {
			return resolved, nil
		}
		return "", fmt.Errorf("publisher %q: environment variable %s for %s is empty", p.ID, environment, label)
	}
	if value == "" {
		return "", fmt.Errorf("publisher %q: %s is required", p.ID, label)
	}
	return value, nil
}

func (p PublisherConfig) newPublisher() (Publisher, error) {
	identifier, err := p.credential(p.Identifier, p.IdentifierEnv, "identifier")
	if err != nil {
		return nil, err
	}
	password, err := p.credential(p.AppPassword, p.AppPasswordEnv, "app password")
	if err != nil {
		return nil, err
	}
	return newBlueskyPublisher(p.ID, identifier, password, strings.TrimRight(p.Service, "/")), nil
}
