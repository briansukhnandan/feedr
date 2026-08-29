package feedr

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type deliveryJournal struct {
	Deliveries map[string]Receipt `json:"deliveries"`
}

func loadJournal(dir string) (deliveryJournal, error) {
	path := filepath.Join(dir, stateDirName, "deliveries.json")
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return deliveryJournal{Deliveries: map[string]Receipt{}}, nil
	}
	if err != nil {
		return deliveryJournal{}, fmt.Errorf("open delivery journal: %w", err)
	}
	defer file.Close()

	var journal deliveryJournal
	if err := json.NewDecoder(file).Decode(&journal); err != nil {
		return deliveryJournal{}, fmt.Errorf("decode delivery journal: %w", err)
	}
	if journal.Deliveries == nil {
		journal.Deliveries = map[string]Receipt{}
	}
	return journal, nil
}

func (j deliveryJournal) save(dir string) error {
	stateDir := filepath.Join(dir, stateDirName)
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return fmt.Errorf("encode delivery journal: %w", err)
	}
	path := filepath.Join(stateDir, "deliveries.json")
	temporary, err := os.CreateTemp(stateDir, ".deliveries-*.json")
	if err != nil {
		return fmt.Errorf("create temporary journal: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if _, err := temporary.Write(append(data, '\n')); err != nil {
		temporary.Close()
		return fmt.Errorf("write delivery journal: %w", err)
	}
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("set delivery journal permissions: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close delivery journal: %w", err)
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return fmt.Errorf("replace delivery journal: %w", err)
	}
	return nil
}
