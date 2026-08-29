// Package feedr contains the file-driven publishing daemon.
package feedr

import (
	"context"
	"time"
)

// Post is the portable JSON contract written by a feed producer.  It mirrors
// the former FeedItem contract, except that media is represented by URLs so a
// producer can create posts.json in any language.
type Post struct {
	ID          string         `json:"id"`
	Text        string         `json:"text"`
	URL         string         `json:"url,omitempty"`
	PublishedAt string         `json:"publishedAt,omitempty"`
	Media       []Media        `json:"media,omitempty"`
	Thread      []PostSegment  `json:"thread,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type PostSegment struct {
	Text  string  `json:"text"`
	URL   string  `json:"url,omitempty"`
	Media []Media `json:"media,omitempty"`
}

type Media struct {
	Alt      string `json:"alt"`
	MIMEType string `json:"mimeType"`
	URL      string `json:"url"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
}

type Receipt struct {
	ID          string         `json:"id"`
	URL         string         `json:"url,omitempty"`
	PublishedAt time.Time      `json:"publishedAt"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// Publisher is deliberately small so new destinations can be implemented
// without coupling a post producer to Go, Node, or another runtime.
type Publisher interface {
	ID() string
	Publish(ctx Context, post Post) (Receipt, error)
}

// Context is kept as an alias to make publisher implementations explicit.
// It permits cancellation when the daemon is stopped.
type Context = context.Context
