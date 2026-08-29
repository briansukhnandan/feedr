package feedr

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigRoutesFeedsToPublisherAccounts(t *testing.T) {
	dir := t.TempDir()
	configJSON := `{
  "timezone": "UTC",
  "publishers": [{
    "id": "bluesky",
    "type": "bluesky",
    "accounts": [
      {"id": "reddit", "identifier": "reddit.example", "appPassword": "reddit-password"},
      {"id": "congress", "identifier": "congress.example", "appPassword": "congress-password"}
    ]
  }],
  "feeds": [
    {"id": "reddit", "destinations": [{"publisher": "bluesky", "account": "reddit"}]},
    {"id": "congress", "destinations": [{"publisher": "bluesky", "account": "congress"}]}
  ]
}`
	if err := os.WriteFile(filepath.Join(dir, configFileName), []byte(configJSON), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := readConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	feed, found := config.feed("congress")
	if !found || len(feed.Destinations) != 1 || feed.Destinations[0].Account != "congress" {
		t.Fatalf("congress route = %#v, found = %t", feed.Destinations, found)
	}
	publisher, err := config.Publishers[0].newPublisher(config.Publishers[0].Accounts[1])
	if err != nil {
		t.Fatal(err)
	}
	if publisher.ID() != "bluesky:congress" {
		t.Fatalf("publisher id = %q", publisher.ID())
	}
}
