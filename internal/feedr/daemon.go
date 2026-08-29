package feedr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

type Daemon struct {
	dir    string
	logger *slog.Logger
	now    func() time.Time
}

func NewDaemon(dir string, logger *slog.Logger) *Daemon {
	return &Daemon{dir: dir, logger: logger, now: time.Now}
}

// Run processes the current day's file immediately and then polls it. It
// reloads configuration and posts on every pass, making configuration changes
// effective without restarting the container.
func (d *Daemon) Run(ctx context.Context) error {
	for {
		config, err := readConfig(d.dir)
		if err != nil {
			d.logger.Error("configuration is invalid; will retry", "error", err)
		} else {
			d.processCurrentDay(ctx, config)
		}

		interval := 60 * time.Second
		if err == nil {
			interval = time.Duration(config.PollIntervalSeconds) * time.Second
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (d *Daemon) processCurrentDay(ctx context.Context, config Config) {
	date := d.now().In(config.location()).Format("2006_01_02")
	journal, err := loadJournal(d.dir)
	if err != nil {
		d.logger.Error("could not load delivery journal", "error", err)
		return
	}
	publishers := d.newPublishers(config)

	if config.DefaultFeed != "" {
		feed, _ := config.feed(config.DefaultFeed)
		d.processFeedFile(ctx, date, feed, filepath.Join(d.dir, date, "posts.json"), publishers, &journal)
	}
	d.processNodeFiles(ctx, date, config, publishers, &journal)
}

func (d *Daemon) newPublishers(config Config) map[string]Publisher {
	publishers := make(map[string]Publisher)
	for _, publisherConfig := range config.Publishers {
		for _, account := range publisherConfig.Accounts {
			publisher, err := publisherConfig.newPublisher(account)
			if err != nil {
				d.logger.Error("publisher account is unavailable", "publisher", publisherConfig.ID, "account", account.ID, "error", err)
				continue
			}
			publishers[destinationKey(DestinationConfig{Publisher: publisherConfig.ID, Account: account.ID})] = publisher
		}
	}
	return publishers
}

func (d *Daemon) processNodeFiles(ctx context.Context, date string, config Config, publishers map[string]Publisher, journal *deliveryJournal) {
	nodesDir := filepath.Join(d.dir, date, "nodes")
	entries, err := os.ReadDir(nodesDir)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		d.logger.Error("could not read node directory", "date", date, "error", err)
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		feed, configured := config.feed(entry.Name())
		if !configured {
			d.logger.Warn("ignoring node with no configured feed", "node", entry.Name())
			continue
		}
		d.processFeedFile(ctx, date, feed, filepath.Join(nodesDir, entry.Name(), "posts.json"), publishers, journal)
	}
}

func (d *Daemon) processFeedFile(ctx context.Context, date string, feed FeedConfig, path string, publishers map[string]Publisher, journal *deliveryJournal) {
	posts, found, err := d.readPosts(path)
	if err != nil {
		d.logger.Error("could not read posts", "feed", feed.ID, "path", path, "error", err)
		return
	}
	if !found {
		return
	}
	for _, post := range posts {
		for _, destination := range feed.Destinations {
			publisher, available := publishers[destinationKey(destination)]
			if !available {
				d.logger.Error("destination account is unavailable", "feed", feed.ID, "publisher", destination.Publisher, "account", destination.Account)
				continue
			}
			key := date + "|" + feed.ID + "|" + post.ID + "|" + publisher.ID()
			if _, delivered := journal.Deliveries[key]; delivered {
				continue
			}
			receipt, err := publisher.Publish(ctx, post)
			if err != nil {
				d.logger.Error("publication failed", "post", post.ID, "publisher", publisher.ID(), "error", err)
				continue
			}
			journal.Deliveries[key] = receipt
			if err := journal.save(d.dir); err != nil {
				d.logger.Error("published but could not save delivery journal; retry may duplicate the post", "post", post.ID, "publisher", publisher.ID(), "error", err)
				continue
			}
			d.logger.Info("published post", "feed", feed.ID, "post", post.ID, "publisher", publisher.ID(), "receipt", receipt.ID)
		}
	}
}

func (d *Daemon) readPosts(path string) ([]Post, bool, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var posts []Post
	if err := decoder.Decode(&posts); err != nil {
		return nil, true, fmt.Errorf("decode %s: %w", path, err)
	}
	seen := make(map[string]bool, len(posts))
	for index, post := range posts {
		if post.ID == "" {
			return nil, true, fmt.Errorf("post %d has no id", index)
		}
		if seen[post.ID] {
			return nil, true, fmt.Errorf("post id %q is duplicated", post.ID)
		}
		seen[post.ID] = true
		if err := validatePost(post); err != nil {
			return nil, true, fmt.Errorf("post %q: %w", post.ID, err)
		}
	}
	return posts, true, nil
}

func validatePost(post Post) error {
	for _, segment := range post.Thread {
		for _, image := range segment.Media {
			if image.URL == "" || image.MIMEType == "" || image.Alt == "" {
				return fmt.Errorf("each media item requires alt, url, and mimeType")
			}
		}
	}
	for _, image := range post.Media {
		if image.URL == "" || image.MIMEType == "" || image.Alt == "" {
			return fmt.Errorf("each media item requires alt, url, and mimeType")
		}
	}
	return nil
}
