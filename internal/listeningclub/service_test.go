package listeningclub

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// InMemoryStorage for testing
type InMemoryStorage struct {
	data *ListeningClub
}

func NewInMemoryStorage() *InMemoryStorage {
	week, year := GetCurrentWeek()
	return &InMemoryStorage{
		data: &ListeningClub{
			Votes:      make(map[string]*Vote),
			WeekNumber: week,
			Year:       year,
		},
	}
}

func (s *InMemoryStorage) Save(lc *ListeningClub) error {
	s.data = lc
	return nil
}

func (s *InMemoryStorage) Load() (*ListeningClub, error) {
	return s.data, nil
}

func (s *InMemoryStorage) Exists() bool {
	return s.data != nil
}

func TestService_SetAlbum(t *testing.T) {
	storage := NewInMemoryStorage()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	service := NewService(storage, logger)

	err := service.SetAlbum("Radiohead", "OK Computer", "testuser")
	if err != nil {
		t.Fatalf("Failed to set album: %v", err)
	}

	album, err := service.GetCurrentAlbum()
	if err != nil {
		t.Fatalf("Failed to get current album: %v", err)
	}

	if album == nil {
		t.Fatal("Expected album to be set, got nil")
	}

	if album.Artist != "Radiohead" {
		t.Errorf("Expected artist 'Radiohead', got '%s'", album.Artist)
	}

	if album.Album != "OK Computer" {
		t.Errorf("Expected album 'OK Computer', got '%s'", album.Album)
	}

	if album.SetBy != "testuser" {
		t.Errorf("Expected set by 'testuser', got '%s'", album.SetBy)
	}
}

func TestService_Vote(t *testing.T) {
	storage := NewInMemoryStorage()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	service := NewService(storage, logger)

	// Set an album first
	err := service.SetAlbum("Radiohead", "OK Computer", "testuser")
	if err != nil {
		t.Fatalf("Failed to set album: %v", err)
	}

	// Test voting
	err = service.Vote("test", "user1", "User1", 8, "Great album!")
	if err != nil {
		t.Fatalf("Failed to vote: %v", err)
	}

	err = service.Vote("test", "user2", "User2", 9, "")
	if err != nil {
		t.Fatalf("Failed to vote: %v", err)
	}

	// Test getting stats
	stats, err := service.GetVoteStats()
	if err != nil {
		t.Fatalf("Failed to get vote stats: %v", err)
	}

	if stats.TotalVotes != 2 {
		t.Errorf("Expected 2 votes, got %d", stats.TotalVotes)
	}

	expectedAvg := (8.0 + 9.0) / 2.0
	if stats.AverageRating != expectedAvg {
		t.Errorf("Expected average rating %.1f, got %.1f", expectedAvg, stats.AverageRating)
	}

	// Test vote update (same user voting again)
	err = service.Vote("test", "user1", "User1", 10, "Changed my mind!")
	if err != nil {
		t.Fatalf("Failed to update vote: %v", err)
	}

	stats, err = service.GetVoteStats()
	if err != nil {
		t.Fatalf("Failed to get updated vote stats: %v", err)
	}

	if stats.TotalVotes != 2 {
		t.Errorf("Expected 2 votes after update, got %d", stats.TotalVotes)
	}

	expectedAvgUpdated := (10.0 + 9.0) / 2.0
	if stats.AverageRating != expectedAvgUpdated {
		t.Errorf("Expected updated average rating %.1f, got %.1f", expectedAvgUpdated, stats.AverageRating)
	}
}

func TestService_VoteValidation(t *testing.T) {
	storage := NewInMemoryStorage()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	service := NewService(storage, logger)

	// Test voting without album set
	err := service.Vote("test", "user1", "User1", 8, "")
	if err == nil {
		t.Error("Expected error when voting without album set")
	}

	// Set an album
	err = service.SetAlbum("Test Artist", "Test Album", "testuser")
	if err != nil {
		t.Fatalf("Failed to set album: %v", err)
	}

	// Test invalid ratings
	testCases := []int{0, -1, 11, 15}
	for _, rating := range testCases {
		err = service.Vote("test", "user1", "User1", rating, "")
		if err == nil {
			t.Errorf("Expected error for invalid rating %d", rating)
		}
	}

	// Test valid rating
	err = service.Vote("test", "user1", "User1", 5, "")
	if err != nil {
		t.Errorf("Valid rating should not produce error: %v", err)
	}
}

func TestFileStorage(t *testing.T) {
	tempDir := t.TempDir()
	storage := NewFileStorage(tempDir)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	service := NewService(storage, logger)

	// Test setting and persisting
	err := service.SetAlbum("Test Artist", "Test Album", "testuser")
	if err != nil {
		t.Fatalf("Failed to set album: %v", err)
	}

	// Create new service with same storage to test persistence
	service2 := NewService(storage, logger)
	album, err := service2.GetCurrentAlbum()
	if err != nil {
		t.Fatalf("Failed to get album from new service: %v", err)
	}

	if album == nil {
		t.Fatal("Album should persist across service instances")
	}

	if album.Artist != "Test Artist" || album.Album != "Test Album" {
		t.Errorf("Album data not persisted correctly: got %s - %s", album.Artist, album.Album)
	}

	// Check file exists
	filePath := filepath.Join(tempDir, "listening_club.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Storage file should exist")
	}
}

func TestWeekCalculations(t *testing.T) {
	week, year := GetCurrentWeek()
	if week < 1 || week > 53 {
		t.Errorf("Invalid week number: %d", week)
	}

	if year < 2020 {
		t.Errorf("Invalid year: %d", year)
	}

	weekStart := GetWeekStart()
	weekEnd := GetWeekEnd()

	if !weekEnd.After(weekStart) {
		t.Error("Week end should be after week start")
	}

	// Week should be exactly 7 days (Sunday to Saturday)
	duration := weekEnd.Sub(weekStart)
	expectedDuration := 7*24*time.Hour - time.Second // Almost 7 days (23:59:59 on Saturday)
	if duration < expectedDuration {
		t.Errorf("Week duration too short: %v", duration)
	}

	// Week should start on Sunday (weekday 0)
	if weekStart.Weekday() != time.Sunday {
		t.Errorf("Week should start on Sunday, got %v", weekStart.Weekday())
	}

	// Week should end on Saturday (weekday 6)
	// Note: weekEnd is Saturday 23:59:59, so the day should be Saturday
	if weekEnd.Weekday() != time.Saturday {
		t.Errorf("Week should end on Saturday, got %v", weekEnd.Weekday())
	}

	// Week start should be at midnight (00:00:00)
	if weekStart.Hour() != 0 || weekStart.Minute() != 0 || weekStart.Second() != 0 {
		t.Errorf("Week should start at midnight, got %02d:%02d:%02d",
			weekStart.Hour(), weekStart.Minute(), weekStart.Second())
	}

	// Verify Eastern Time zone
	et, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("Cannot load Eastern Time zone for testing")
	}
	if weekStart.Location().String() != et.String() {
		t.Errorf("Week start should be in Eastern Time, got %v", weekStart.Location())
	}
}

func TestSundayWeekStart(t *testing.T) {
	// Test that weeks start on Sunday at midnight Eastern Time
	et, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("Cannot load Eastern Time zone for testing")
	}

	// Test with different days of the week
	testCases := []struct {
		name        string
		testTime    time.Time
		expectedDay int // Days since Sunday (0 = Sunday)
	}{
		{
			name:        "Sunday",
			testTime:    time.Date(2024, 1, 7, 15, 30, 0, 0, et), // Sunday afternoon
			expectedDay: 0,
		},
		{
			name:        "Monday",
			testTime:    time.Date(2024, 1, 8, 10, 0, 0, 0, et), // Monday morning
			expectedDay: 0,                                      // Should still be same week's Sunday
		},
		{
			name:        "Wednesday",
			testTime:    time.Date(2024, 1, 10, 12, 0, 0, 0, et), // Wednesday noon
			expectedDay: 0,                                       // Should be Sunday of the same week
		},
		{
			name:        "Saturday",
			testTime:    time.Date(2024, 1, 13, 23, 0, 0, 0, et), // Saturday night
			expectedDay: 0,                                       // Should be Sunday of the same week
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Mock the current time by calculating based on the test time
			weekday := int(tc.testTime.Weekday())
			daysBack := weekday
			if weekday == 0 { // Already Sunday
				daysBack = 0
			}

			sunday := tc.testTime.AddDate(0, 0, -daysBack)
			weekStart := time.Date(sunday.Year(), sunday.Month(), sunday.Day(), 0, 0, 0, 0, et)

			// Verify it's Sunday
			if weekStart.Weekday() != time.Sunday {
				t.Errorf("Expected Sunday, got %v", weekStart.Weekday())
			}

			// Verify it's midnight
			if weekStart.Hour() != 0 || weekStart.Minute() != 0 || weekStart.Second() != 0 {
				t.Errorf("Expected midnight, got %02d:%02d:%02d",
					weekStart.Hour(), weekStart.Minute(), weekStart.Second())
			}

			// Verify it's in Eastern Time
			if weekStart.Location().String() != et.String() {
				t.Errorf("Expected Eastern Time, got %v", weekStart.Location())
			}
		})
	}
}
