package feedr

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// deliveryStore keeps one row per successful delivery. Its primary key is the
// same composite string used by the former JSON journal.
type deliveryStore struct {
	db *sql.DB
}

func openDeliveryStore(dir string) (*deliveryStore, error) {
	stateDir := filepath.Join(dir, stateDirName)
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return nil, fmt.Errorf("create delivery state directory: %w", err)
	}

	databasePath := filepath.Join(stateDir, "deliveries.db")
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, fmt.Errorf("open delivery database: %w", err)
	}
	db.SetMaxOpenConns(1)

	store := &deliveryStore{db: db}
	if err := store.initialize(); err != nil {
		db.Close()
		return nil, err
	}
	if err := os.Chmod(databasePath, 0o600); err != nil {
		db.Close()
		return nil, fmt.Errorf("set delivery database permissions: %w", err)
	}
	return store, nil
}

func (s *deliveryStore) initialize() error {
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS deliveries (
			delivery_key TEXT PRIMARY KEY,
			receipt_json TEXT NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("create deliveries table: %w", err)
	}
	return nil
}

func (s *deliveryStore) delivered(key string) (bool, error) {
	var exists bool
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM deliveries WHERE delivery_key = ?)`, key).Scan(&exists); err != nil {
		return false, fmt.Errorf("check delivery: %w", err)
	}
	return exists, nil
}

func (s *deliveryStore) record(key string, receipt Receipt) error {
	data, err := json.Marshal(receipt)
	if err != nil {
		return fmt.Errorf("encode delivery receipt: %w", err)
	}
	if _, err := s.db.Exec(`INSERT INTO deliveries (delivery_key, receipt_json) VALUES (?, ?)`, key, data); err != nil {
		return fmt.Errorf("record delivery: %w", err)
	}
	return nil
}

func (s *deliveryStore) close() error {
	return s.db.Close()
}
