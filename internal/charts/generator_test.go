package charts

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/syakter/go-chuu/internal/types"
	"github.com/syakter/go-lastfm/lastfm"
)

func TestGenerator_CreateAlbumChart(t *testing.T) {
	// Setup logger that discards output for tests
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create temp directory for test
	tempDir := filepath.Join(os.TempDir(), "test-charts")
	lastfmAPI := lastfm.New("test-key", "test-secret")

	// Create generator
	generator := NewGenerator(logger, tempDir, lastfmAPI)

	// Test EnsureTempDir
	err := generator.EnsureTempDir()
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Test with mock album data
	mockAlbums := []types.Album{
		{
			Name:   "Test Album 1",
			Artist: "Test Artist 1",
			Image:  "", // Empty image URL to test placeholder functionality
		},
		{
			Name:   "Test Album 2",
			Artist: "Test Artist 2",
			Image:  "https://httpbin.org/image/png", // Test with a real image URL
		},
	}

	ctx := context.Background()
	chartImage, err := generator.createAlbumChart(ctx, mockAlbums)
	if err != nil {
		t.Fatalf("Failed to create album chart: %v", err)
	}

	if chartImage == nil {
		t.Fatal("Chart image is nil")
	}

	// Check image dimensions
	bounds := chartImage.Bounds()
	if bounds.Dx() != 900 || bounds.Dy() != 900 {
		t.Errorf("Expected image dimensions 900x900, got %dx%d", bounds.Dx(), bounds.Dy())
	}

	// Clean up
	os.RemoveAll(tempDir)
}

func TestGenerator_FormatPeriodForAPI(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "test-charts")
	lastfmAPI := lastfm.New("test-key", "test-secret")
	generator := NewGenerator(nil, tempDir, lastfmAPI)

	tests := []struct {
		input    string
		expected string
	}{
		{"7d", "7day"},
		{"1w", "7day"},
		{"1m", "1month"},
		{"30d", "1month"},
		{"3m", "3month"},
		{"90d", "3month"},
		{"6m", "6month"},
		{"180d", "6month"},
		{"1y", "12month"},
		{"365d", "12month"},
		{"overall", "overall"},
		{"invalid", "overall"},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			result := generator.formatPeriodForAPI(test.input)
			if result != test.expected {
				t.Errorf("Expected %s, got %s", test.expected, result)
			}
		})
	}
}

func TestGenerator_DownloadAlbumArt(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	tempDir := filepath.Join(os.TempDir(), "test-charts")
	lastfmAPI := lastfm.New("test-key", "test-secret")
	generator := NewGenerator(logger, tempDir, lastfmAPI)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test with empty URL
	_, err := generator.downloadAlbumArt(ctx, "")
	if err == nil {
		t.Error("Expected error for empty image URL")
	}

	// Test with valid image URL (httpbin provides test images)
	img, err := generator.downloadAlbumArt(ctx, "https://httpbin.org/image/png")
	if err != nil {
		t.Errorf("Failed to download valid image: %v", err)
	} else if img == nil {
		t.Error("Downloaded image is nil")
	}
}
