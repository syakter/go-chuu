package listeningclub

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Storage interface for listening club data persistence
type Storage interface {
	Save(lc *ListeningClub) error
	Load() (*ListeningClub, error)
	Exists() bool
}

// FileStorage implements Storage using JSON files
type FileStorage struct {
	filePath string
	mu       sync.RWMutex
}

// NewFileStorage creates a new file-based storage
func NewFileStorage(dataDir string) *FileStorage {
	// Ensure data directory exists
	os.MkdirAll(dataDir, 0755)

	return &FileStorage{
		filePath: filepath.Join(dataDir, "listening_club.json"),
	}
}

// Save saves the listening club data to file
func (fs *FileStorage) Save(lc *ListeningClub) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	data, err := json.MarshalIndent(lc, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal listening club data: %w", err)
	}

	if err := os.WriteFile(fs.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write listening club data: %w", err)
	}

	return nil
}

// Load loads the listening club data from file
func (fs *FileStorage) Load() (*ListeningClub, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	if !fs.exists() {
		// Return empty listening club if file doesn't exist
		week, year := GetCurrentWeek()
		return &ListeningClub{
			Votes:      make(map[string]*Vote),
			WeekNumber: week,
			Year:       year,
		}, nil
	}

	data, err := os.ReadFile(fs.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read listening club data: %w", err)
	}

	var lc ListeningClub
	if err := json.Unmarshal(data, &lc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal listening club data: %w", err)
	}

	// Initialize votes map if nil
	if lc.Votes == nil {
		lc.Votes = make(map[string]*Vote)
	}

	return &lc, nil
}

// Exists checks if the storage file exists
func (fs *FileStorage) Exists() bool {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	return fs.exists()
}

func (fs *FileStorage) exists() bool {
	_, err := os.Stat(fs.filePath)
	return err == nil
}
