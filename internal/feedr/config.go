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
	stateDirName   = "state"
)

type Config struct {
	PollIntervalSeconds int               `json:"pollIntervalSeconds,omitempty"`
	Timezone            string            `json:"timezone,omitempty"`
	DefaultFeed         string            `json:"defaultFeed,omitempty"`
	Publishers          []PublisherConfig `json:"publishers"`
	Feeds               []FeedConfig      `json:"feeds"`
}

type PublisherConfig struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Accounts []AccountConfig `json:"accounts"`
}

type AccountConfig struct {
	ID             string `json:"id"`
	Identifier     string `json:"identifier,omitempty"`
	IdentifierEnv  string `json:"identifierEnv,omitempty"`
	AppPassword    string `json:"appPassword,omitempty"`
	AppPasswordEnv string `json:"appPasswordEnv,omitempty"`
	Service        string `json:"service,omitempty"`
}

type FeedConfig struct {
	ID           string              `json:"id"`
	Destinations []DestinationConfig `json:"destinations"`
}

type DestinationConfig struct {
	Publisher string `json:"publisher"`
	Account   string `json:"account"`
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
	if len(config.Feeds) == 0 {
		return Config{}, fmt.Errorf("configuration has no feeds")
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

	publishers := make(map[string]PublisherConfig, len(config.Publishers))
	for i, publisher := range config.Publishers {
		if publisher.ID == "" || publisher.Type == "" {
			return Config{}, fmt.Errorf("publisher %d requires id and type", i)
		}
		if _, exists := publishers[publisher.ID]; exists {
			return Config{}, fmt.Errorf("publisher id %q is duplicated", publisher.ID)
		}
		if publisher.Type != "bluesky" {
			return Config{}, fmt.Errorf("publisher %q has unsupported type %q", publisher.ID, publisher.Type)
		}
		if len(publisher.Accounts) == 0 {
			return Config{}, fmt.Errorf("publisher %q has no accounts", publisher.ID)
		}
		accounts := make(map[string]bool, len(publisher.Accounts))
		for accountIndex, account := range publisher.Accounts {
			if account.ID == "" {
				return Config{}, fmt.Errorf("publisher %q account %d requires id", publisher.ID, accountIndex)
			}
			if accounts[account.ID] {
				return Config{}, fmt.Errorf("publisher %q account id %q is duplicated", publisher.ID, account.ID)
			}
			accounts[account.ID] = true
		}
		publishers[publisher.ID] = publisher
	}

	feeds := make(map[string]bool, len(config.Feeds))
	for index, feed := range config.Feeds {
		if feed.ID == "" {
			return Config{}, fmt.Errorf("feed %d requires id", index)
		}
		if feeds[feed.ID] {
			return Config{}, fmt.Errorf("feed id %q is duplicated", feed.ID)
		}
		feeds[feed.ID] = true
		if len(feed.Destinations) == 0 {
			return Config{}, fmt.Errorf("feed %q has no destinations", feed.ID)
		}
		for _, destination := range feed.Destinations {
			publisher, exists := publishers[destination.Publisher]
			if !exists {
				return Config{}, fmt.Errorf("feed %q references unknown publisher %q", feed.ID, destination.Publisher)
			}
			if !publisher.hasAccount(destination.Account) {
				return Config{}, fmt.Errorf("feed %q references unknown account %q on publisher %q", feed.ID, destination.Account, destination.Publisher)
			}
		}
	}
	if config.DefaultFeed == "" {
		return Config{}, fmt.Errorf("defaultFeed is required")
	}
	if !feeds[config.DefaultFeed] {
		return Config{}, fmt.Errorf("defaultFeed %q is not configured", config.DefaultFeed)
	}
	return config, nil
}

func (c Config) location() *time.Location {
	// readConfig has already validated this value.
	location, _ := time.LoadLocation(c.Timezone)
	return location
}

func (p PublisherConfig) hasAccount(id string) bool {
	for _, account := range p.Accounts {
		if account.ID == id {
			return true
		}
	}
	return false
}

func (a AccountConfig) credential(publisherID, value, environment, label string) (string, error) {
	if environment != "" {
		if resolved := os.Getenv(environment); resolved != "" {
			return resolved, nil
		}
		return "", fmt.Errorf("publisher %q account %q: environment variable %s for %s is empty", publisherID, a.ID, environment, label)
	}
	if value == "" {
		return "", fmt.Errorf("publisher %q account %q: %s is required", publisherID, a.ID, label)
	}
	return value, nil
}

func (p PublisherConfig) newPublisher(account AccountConfig) (Publisher, error) {
	identifier, err := account.credential(p.ID, account.Identifier, account.IdentifierEnv, "identifier")
	if err != nil {
		return nil, err
	}
	password, err := account.credential(p.ID, account.AppPassword, account.AppPasswordEnv, "app password")
	if err != nil {
		return nil, err
	}
	return newBlueskyPublisher(p.ID+":"+account.ID, identifier, password, strings.TrimRight(account.Service, "/")), nil
}

func (c Config) feed(id string) (FeedConfig, bool) {
	for _, feed := range c.Feeds {
		if feed.ID == id {
			return feed, true
		}
	}
	return FeedConfig{}, false
}

func destinationKey(destination DestinationConfig) string {
	return destination.Publisher + "\x00" + destination.Account
}
