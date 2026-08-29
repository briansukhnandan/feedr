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
	posts, found, err := d.readPosts(date)
	if err != nil {
		d.logger.Error("could not read posts", "date", date, "error", err)
		return
	}
	if !found {
		return
	}

	publishers := make([]Publisher, 0, len(config.Publishers))
	for _, publisherConfig := range config.Publishers {
		publisher, err := publisherConfig.newPublisher()
		if err != nil {
			d.logger.Error("publisher is unavailable", "publisher", publisherConfig.ID, "error", err)
			continue
		}
		publishers = append(publishers, publisher)
	}
	if len(publishers) == 0 {
		d.logger.Error("no configured publisher is available")
		return
	}

	journal, err := loadJournal(d.dir)
	if err != nil {
		d.logger.Error("could not load delivery journal", "error", err)
		return
	}
	for _, post := range posts {
		for _, publisher := range publishers {
			key := date + "|" + post.ID + "|" + publisher.ID()
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
			d.logger.Info("published post", "post", post.ID, "publisher", publisher.ID(), "receipt", receipt.ID)
		}
	}
}

func (d *Daemon) readPosts(date string) ([]Post, bool, error) {
	path := filepath.Join(d.dir, date, "posts.json")
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
