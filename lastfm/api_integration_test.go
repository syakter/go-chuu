package lastfm

import (
	"os"
	"testing"
)

func TestIntegrationLastFM(t *testing.T) {
	apiKey := os.Getenv("LASTFM_API_KEY")
	apiSecret := os.Getenv("LASTFM_API_SECRET")
	testUser := os.Getenv("LASTFM_TEST_USER")

	if apiKey == "" || apiSecret == "" || testUser == "" {
		t.Skip("Skipping integration test: LASTFM_API_KEY, LASTFM_API_SECRET, and LASTFM_TEST_USER must be set")
	}

	api := New(apiKey, apiSecret)

	t.Run("GetRecentTracks", func(t *testing.T) {
		result, err := api.User.GetRecentTracks(P{
			"user":  testUser,
			"limit": 2,
		})
		if err != nil {
			t.Fatalf("GetRecentTracks failed: %v", err)
		}
		if len(result.Tracks) == 0 {
			t.Error("Expected at least one track")
		}
		if result.Tracks[0].Name == "" {
			t.Error("Track name should not be empty")
		}
		if result.Tracks[0].Artist.Name == "" {
			t.Error("Artist name should not be empty")
		}
	})

	t.Run("GetTopArtists", func(t *testing.T) {
		result, err := api.User.GetTopArtists(P{
			"user":   testUser,
			"period": "7day",
			"limit":  5,
		})
		if err != nil {
			t.Fatalf("GetTopArtists failed: %v", err)
		}
		if len(result.Artists) == 0 {
			t.Error("Expected at least one artist")
		}
		if result.Artists[0].Name == "" {
			t.Error("Artist name should not be empty")
		}
	})
}
