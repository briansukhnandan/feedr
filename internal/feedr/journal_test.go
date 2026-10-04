package feedr

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDeliveryStoreRecordsDeliveries(t *testing.T) {
	dir := t.TempDir()
	store, err := openDeliveryStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, stateDirName, "deliveries.db")); err != nil {
		t.Fatalf("delivery database was not created: %v", err)
	}
	key := "2026_10_03|news|item-42|bluesky:main"
	if err := store.record(key, Receipt{ID: "at://example/post", PublishedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if err := store.close(); err != nil {
		t.Fatal(err)
	}

	store, err = openDeliveryStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.close()
	delivered, err := store.delivered(key)
	if err != nil {
		t.Fatal(err)
	}
	if !delivered {
		t.Fatal("recorded delivery was not found after reopening")
	}
}
