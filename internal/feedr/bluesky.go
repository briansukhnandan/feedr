package feedr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultBlueskyService = "https://bsky.social"
	maxBlueskyRunes       = 300
	maxImageBytes          = 10 << 20
)

type blueskyPublisher struct {
	id         string
	identifier string
	password   string
	service    string
	client     *http.Client
	session    *blueskySession
}

type blueskySession struct {
	AccessJWT string `json:"accessJwt"`
	DID       string `json:"did"`
}

type blueskyStrongRef struct {
	URI string `json:"uri"`
	CID string `json:"cid"`
}

type blueskyRecordResult struct {
	URI string `json:"uri"`
	CID string `json:"cid"`
}

func newBlueskyPublisher(id, identifier, password, service string) *blueskyPublisher {
	if service == "" {
		service = defaultBlueskyService
	}
	return &blueskyPublisher{
		id:         id,
		identifier: identifier,
		password:   password,
		service:    service,
		client:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *blueskyPublisher) ID() string { return p.id }

func (p *blueskyPublisher) Publish(ctx context.Context, post Post) (Receipt, error) {
	session, err := p.getSession(ctx)
	if err != nil {
		return Receipt{}, err
	}
	segments := post.Thread
	if len(segments) == 0 {
		segments = []PostSegment{{Text: post.Text, URL: post.URL, Media: post.Media}}
	}

	var root, parent *blueskyStrongRef
	var last blueskyStrongRef
	for _, segment := range segments {
		result, err := p.createPost(ctx, session, segment, root, parent)
		if err != nil {
			return Receipt{}, err
		}
		ref := &blueskyStrongRef{URI: result.URI, CID: result.CID}
		if root == nil {
			root = ref
		}
		parent = ref
		last = *ref
	}

	postID := last.URI[strings.LastIndex(last.URI, "/")+1:]
	return Receipt{
		ID:          last.URI,
		URL:         fmt.Sprintf("https://bsky.app/profile/%s/post/%s", session.DID, postID),
		PublishedAt: time.Now().UTC(),
		Metadata:    map[string]any{"rootUri": root.URI},
	}, nil
}

func (p *blueskyPublisher) getSession(ctx context.Context) (blueskySession, error) {
	if p.session != nil {
		return *p.session, nil
	}
	session, err := p.createSession(ctx)
	if err != nil {
		return blueskySession{}, err
	}
	p.session = &session
	return session, nil
}

func (p *blueskyPublisher) createSession(ctx context.Context) (blueskySession, error) {
	var session blueskySession
	err := p.jsonRequest(ctx, http.MethodPost, "/xrpc/com.atproto.server.createSession", "", map[string]string{
		"identifier": p.identifier,
		"password":   p.password,
	}, &session)
	if err != nil {
		return blueskySession{}, fmt.Errorf("create Bluesky session: %w", err)
	}
	if session.AccessJWT == "" || session.DID == "" {
		return blueskySession{}, fmt.Errorf("create Bluesky session: response omitted credentials")
	}
	return session, nil
}

func (p *blueskyPublisher) createPost(ctx context.Context, session blueskySession, segment PostSegment, root, parent *blueskyStrongRef) (blueskyRecordResult, error) {
	text, facet := blueskyText(segment.Text, segment.URL)
	record := map[string]any{
		"$type":     "app.bsky.feed.post",
		"text":      text,
		"createdAt": time.Now().UTC().Format(time.RFC3339Nano),
	}
	if facet != nil {
		record["facets"] = []any{facet}
	}
	if root != nil && parent != nil {
		record["reply"] = map[string]any{"root": root, "parent": parent}
	}
	if len(segment.Media) > 0 {
		embed, err := p.imageEmbed(ctx, session.AccessJWT, segment.Media)
		if err != nil {
			return blueskyRecordResult{}, err
		}
		record["embed"] = embed
	}

	var result blueskyRecordResult
	err := p.jsonRequest(ctx, http.MethodPost, "/xrpc/com.atproto.repo.createRecord", session.AccessJWT, map[string]any{
		"repo":       session.DID,
		"collection": "app.bsky.feed.post",
		"record":     record,
	}, &result)
	if err != nil {
		return blueskyRecordResult{}, fmt.Errorf("create Bluesky post: %w", err)
	}
	if result.URI == "" || result.CID == "" {
		return blueskyRecordResult{}, fmt.Errorf("create Bluesky post: response omitted uri or cid")
	}
	return result, nil
}

func (p *blueskyPublisher) imageEmbed(ctx context.Context, token string, media []Media) (map[string]any, error) {
	images := make([]any, 0, min(4, len(media)))
	for _, image := range media[:min(4, len(media))] {
		blob, err := p.uploadImage(ctx, token, image)
		if err != nil {
			return nil, err
		}
		entry := map[string]any{"alt": image.Alt, "image": blob}
		if image.Width > 0 && image.Height > 0 {
			entry["aspectRatio"] = map[string]int{"width": image.Width, "height": image.Height}
		}
		images = append(images, entry)
	}
	return map[string]any{"$type": "app.bsky.embed.images", "images": images}, nil
}

func (p *blueskyPublisher) uploadImage(ctx context.Context, token string, image Media) (json.RawMessage, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, image.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("create image request: %w", err)
	}
	response, err := p.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch image %q: %w", image.URL, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch image %q: %s", image.URL, response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxImageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read image %q: %w", image.URL, err)
	}
	if len(data) > maxImageBytes {
		return nil, fmt.Errorf("image %q exceeds the %d byte limit", image.URL, maxImageBytes)
	}

	request, err = http.NewRequestWithContext(ctx, http.MethodPost, p.service+"/xrpc/com.atproto.repo.uploadBlob", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create upload request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", image.MIMEType)
	response, err = p.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("upload image: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("upload image: %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	var payload struct {
		Blob json.RawMessage `json:"blob"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode uploaded image: %w", err)
	}
	if len(payload.Blob) == 0 {
		return nil, fmt.Errorf("upload image: response omitted blob")
	}
	return payload.Blob, nil
}

func (p *blueskyPublisher) jsonRequest(ctx context.Context, method, path, token string, input any, output any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, method, p.service+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := p.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("%s: %s", response.Status, strings.TrimSpace(string(message)))
	}
	return json.NewDecoder(response.Body).Decode(output)
}

// blueskyText adds a link as a byte-indexed rich-text facet. URL facets work
// on bytes, while the post limit is measured in Unicode code points.
func blueskyText(body, url string) (string, map[string]any) {
	if url == "" {
		return truncateRunes(body, maxBlueskyRunes), nil
	}
	suffix := url
	if body != "" {
		suffix = "\n" + url
	}
	if runeCount(suffix) >= maxBlueskyRunes {
		text := truncateRunes(suffix, maxBlueskyRunes)
		return text, nil
	}
	text := truncateRunes(body, maxBlueskyRunes-runeCount(suffix)) + suffix
	start := len(text) - len(url)
	return text, linkFacet(text, start, len(text), url)
}

func linkFacet(_ string, start, end int, url string) map[string]any {
	return map[string]any{
		"index": map[string]int{"byteStart": start, "byteEnd": end},
		"features": []any{map[string]string{"$type": "app.bsky.richtext.facet#link", "uri": url}},
	}
}

func runeCount(value string) int { return len([]rune(value)) }

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
