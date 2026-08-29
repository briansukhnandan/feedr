package feedr

import (
	"strings"
	"testing"
)

func TestBlueskyTextAppendsLinkedURL(t *testing.T) {
	text, facet := blueskyText("Hello", "https://example.com/post")
	if text != "Hello\nhttps://example.com/post" {
		t.Fatalf("text = %q", text)
	}
	index := facet["index"].(map[string]int)
	if index["byteStart"] != len("Hello\n") || index["byteEnd"] != len(text) {
		t.Fatalf("unexpected facet indices: %#v", index)
	}
}

func TestBlueskyTextRespectsRuneLimit(t *testing.T) {
	// A non-ASCII rune ensures this verifies the character limit, not bytes.
	text, facet := blueskyText(strings.Repeat("é", 301), "")
	if facet != nil || runeCount(text) != maxBlueskyRunes {
		t.Fatalf("runes = %d, facet = %#v", runeCount(text), facet)
	}
}
