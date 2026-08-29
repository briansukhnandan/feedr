package feedr

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadPostsAcceptsCurrentFeedItemShape(t *testing.T) {
	dir := t.TempDir()
	day := "2026_08_29"
	if err := os.Mkdir(filepath.Join(dir, day), 0o700); err != nil {
		t.Fatal(err)
	}
	data := `[{"id":"source-1","text":"Hello","url":"https://example.com","metadata":{"source":"test"}}]`
	if err := os.WriteFile(filepath.Join(dir, day, "posts.json"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}

	posts, found, err := NewDaemon(dir, nil).readPosts(day)
	if err != nil {
		t.Fatal(err)
	}
	if !found || len(posts) != 1 || posts[0].ID != "source-1" {
		t.Fatalf("posts = %#v, found = %t", posts, found)
	}
}

func TestReadPostsRejectsMediaWithoutAltText(t *testing.T) {
	dir := t.TempDir()
	day := "2026_08_29"
	if err := os.Mkdir(filepath.Join(dir, day), 0o700); err != nil {
		t.Fatal(err)
	}
	data := `[{"id":"source-1","text":"Hello","media":[{"mimeType":"image/png","url":"https://example.com/image.png"}]}]`
	if err := os.WriteFile(filepath.Join(dir, day, "posts.json"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}

	_, found, err := NewDaemon(dir, nil).readPosts(day)
	if !found || err == nil {
		t.Fatalf("found = %t, err = %v", found, err)
	}
}
