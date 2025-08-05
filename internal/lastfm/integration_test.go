//go:build integration
// +build integration

package lastfm

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/syakter/go-chuu/internal/cache"
	"github.com/syakter/go-chuu/internal/config"
)

// Integration tests require real API credentials
// Run with: go test -tags=integration
// Set environment variables: LASTFM_API_KEY and LASTFM_API_SECRET

func getTestCredentials(t *testing.T) (string, string) {
	apiKey := os.Getenv("LASTFM_API_KEY")
	apiSecret := os.Getenv("LASTFM_API_SECRET")

	if apiKey == "" || apiSecret == "" {
		t.Skip("Skipping integration tests: LASTFM_API_KEY and LASTFM_API_SECRET environment variables required")
	}

	return apiKey, apiSecret
}

func createIntegrationClient(t *testing.T, users []string) *Client {
	apiKey, apiSecret := getTestCredentials(t)

	cfg := &config.Config{
		LastFMAPIKey:          apiKey,
		LastFMAPISecret:       apiSecret,
		MaxConcurrentRequests: 3, // Be gentle with the API
		Users:                 users,
		CacheTTL:              time.Minute,
	}

	cache := cache.NewInMemoryCache(100)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))

	return NewClient(cfg, cache, logger)
}

func TestIntegration_API_GetTopAlbums(t *testing.T) {
	apiKey, apiSecret := getTestCredentials(t)
	api := NewAPI(apiKey, apiSecret)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use a well-known user for testing
	params := map[string]interface{}{
		"user":   "rj", // Last.fm founder, likely to have data
		"period": "7day",
		"limit":  "5",
	}

	result, err := api.GetTopAlbums(ctx, params)
	if err != nil {
		// Check if it's a known API error
		if apiErr, ok := err.(*APIError); ok {
			if apiErr.Code == 6 {
				t.Skip("User not found - this is expected with test data")
			}
			if apiErr.Code == 26 {
				t.Skip("API key invalid - check your credentials")
			}
			if apiErr.Code == 29 {
				t.Skip("Rate limit exceeded - try again later")
			}
		}
		t.Errorf("API call failed: %v", err)
		return
	}

	if result == nil {
		t.Fatal("Expected result but got nil")
	}

	// The response structure should be valid even if no albums are returned
	if result.TopAlbums.Albums == nil {
		t.Error("Expected Albums slice to be initialized")
	}

	t.Logf("Retrieved %d albums", len(result.TopAlbums.Albums))

	// If we got albums, verify structure
	for i, album := range result.TopAlbums.Albums {
		if i >= 3 { // Just check first few
			break
		}

		if album.Name == "" {
			t.Errorf("Album %d has empty name", i)
		}

		if album.Artist.Name == "" {
			t.Errorf("Album %d has empty artist name", i)
		}

		t.Logf("Album %d: %s by %s", i+1, album.Name, album.Artist.Name)
	}
}

func TestIntegration_API_GetTopArtists(t *testing.T) {
	apiKey, apiSecret := getTestCredentials(t)
	api := NewAPI(apiKey, apiSecret)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	params := map[string]interface{}{
		"user":   "rj",
		"period": "7day",
		"limit":  "5",
	}

	result, err := api.GetTopArtists(ctx, params)
	if err != nil {
		if apiErr, ok := err.(*APIError); ok {
			if apiErr.Code == 6 || apiErr.Code == 26 || apiErr.Code == 29 {
				t.Skipf("API error: %v", err)
			}
		}
		t.Errorf("API call failed: %v", err)
		return
	}

	if result == nil {
		t.Fatal("Expected result but got nil")
	}

	t.Logf("Retrieved %d artists", len(result.TopArtists.Artists))

	for i, artist := range result.TopArtists.Artists {
		if i >= 3 {
			break
		}

		if artist.Name == "" {
			t.Errorf("Artist %d has empty name", i)
		}

		t.Logf("Artist %d: %s", i+1, artist.Name)
	}
}

func TestIntegration_API_GetRecentTracks(t *testing.T) {
	apiKey, apiSecret := getTestCredentials(t)
	api := NewAPI(apiKey, apiSecret)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	params := map[string]interface{}{
		"user":  "rj",
		"limit": "5",
	}

	result, err := api.GetRecentTracks(ctx, params)
	if err != nil {
		if apiErr, ok := err.(*APIError); ok {
			if apiErr.Code == 6 || apiErr.Code == 26 || apiErr.Code == 29 {
				t.Skipf("API error: %v", err)
			}
		}
		t.Errorf("API call failed: %v", err)
		return
	}

	if result == nil {
		t.Fatal("Expected result but got nil")
	}

	t.Logf("Retrieved %d recent tracks", len(result.RecentTracks.Tracks))

	for i, track := range result.RecentTracks.Tracks {
		if i >= 3 {
			break
		}

		if track.Name == "" {
			t.Errorf("Track %d has empty name", i)
		}

		if track.Artist.Name == "" {
			t.Errorf("Track %d has empty artist name", i)
		}

		nowPlayingStatus := "not playing"
		if track.NowPlaying == "true" {
			nowPlayingStatus = "NOW PLAYING"
		}

		t.Logf("Track %d: %s by %s [%s]", i+1, track.Name, track.Artist.Name, nowPlayingStatus)
	}
}

func TestIntegration_API_GetArtistInfo(t *testing.T) {
	apiKey, apiSecret := getTestCredentials(t)
	api := NewAPI(apiKey, apiSecret)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	params := map[string]interface{}{
		"artist":   "Radiohead", // Well-known artist
		"username": "rj",
	}

	result, err := api.GetArtistInfo(ctx, params)
	if err != nil {
		if apiErr, ok := err.(*APIError); ok {
			if apiErr.Code == 6 || apiErr.Code == 26 || apiErr.Code == 29 {
				t.Skipf("API error: %v", err)
			}
		}
		t.Errorf("API call failed: %v", err)
		return
	}

	if result == nil {
		t.Fatal("Expected result but got nil")
	}

	if result.Artist.Name == "" {
		t.Error("Expected artist name to be populated")
	}

	t.Logf("Artist: %s, User plays: %s", result.Artist.Name, result.Artist.Stats.UserPlays)
}

func TestIntegration_Client_GetArtistScrobbles(t *testing.T) {
	// Test with a small set of users to avoid hitting rate limits
	client := createIntegrationClient(t, []string{"rj"})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := client.GetArtistScrobbles(ctx, "Radiohead")
	if err != nil {
		if strings.Contains(err.Error(), "user") && strings.Contains(err.Error(), "not found") {
			t.Skip("User not found - expected with test data")
		}
		if strings.Contains(err.Error(), "API key") {
			t.Skip("API key issue - check credentials")
		}
		if strings.Contains(err.Error(), "rate limit") {
			t.Skip("Rate limit exceeded")
		}
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if len(result) != 1 {
		t.Errorf("Expected 1 user result, got %d", len(result))
	}

	for _, userCount := range result {
		if userCount.Username == "" {
			t.Error("Expected username to be populated")
		}

		if userCount.Playcount < 0 {
			t.Error("Expected playcount to be non-negative")
		}

		t.Logf("User %s has %d plays of Radiohead", userCount.Username, userCount.Playcount)
	}
}

func TestIntegration_Client_GetNowPlaying(t *testing.T) {
	client := createIntegrationClient(t, []string{"rj"})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := client.GetNowPlaying(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "user") && strings.Contains(err.Error(), "not found") {
			t.Skip("User not found - expected with test data")
		}
		t.Errorf("Unexpected error: %v", err)
		return
	}

	// Result may be empty if no users are currently playing music
	t.Logf("Found %d users currently playing music", len(result))

	for user, track := range result {
		if user == "" {
			t.Error("Expected username to be populated")
		}

		if track == "" {
			t.Error("Expected track info to be populated")
		}

		t.Logf("User %s is now playing: %s", user, track)
	}
}

func TestIntegration_Client_Caching(t *testing.T) {
	client := createIntegrationClient(t, []string{"rj"})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// First call - should hit the API
	start := time.Now()
	result1, err := client.GetUserTopArtists(ctx, "rj", "7day", 5)
	firstCallDuration := time.Since(start)

	if err != nil {
		if strings.Contains(err.Error(), "user") && strings.Contains(err.Error(), "not found") {
			t.Skip("User not found - expected with test data")
		}
		t.Errorf("First call failed: %v", err)
		return
	}

	// Second call - should use cache and be faster
	start = time.Now()
	result2, err := client.GetUserTopArtists(ctx, "rj", "7day", 5)
	secondCallDuration := time.Since(start)

	if err != nil {
		t.Errorf("Second call failed: %v", err)
		return
	}

	// Results should be identical
	if len(result1) != len(result2) {
		t.Errorf("Expected identical results, got %d vs %d items", len(result1), len(result2))
	}

	// Second call should be significantly faster (cached)
	if secondCallDuration > firstCallDuration/2 {
		t.Logf("Warning: Second call (%v) wasn't significantly faster than first (%v) - caching may not be working", secondCallDuration, firstCallDuration)
	}

	t.Logf("First call: %v, Second call: %v", firstCallDuration, secondCallDuration)
}

func TestIntegration_API_ErrorHandling(t *testing.T) {
	apiKey, apiSecret := getTestCredentials(t)
	api := NewAPI(apiKey, apiSecret)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test with invalid user
	params := map[string]interface{}{
		"user":   "thisusershouldnotexist123456789",
		"period": "7day",
		"limit":  "5",
	}

	_, err := api.GetTopAlbums(ctx, params)

	if err == nil {
		t.Error("Expected error for invalid user but got none")
		return
	}

	// Should be an APIError
	if apiErr, ok := err.(*APIError); ok {
		if apiErr.Code == 6 {
			t.Logf("Got expected 'User not found' error: %v", apiErr)
		} else {
			t.Logf("Got API error with different code: %v", apiErr)
		}
	} else {
		t.Errorf("Expected APIError but got %T: %v", err, err)
	}
}

func TestIntegration_API_RateLimiting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping rate limiting test in short mode")
	}

	apiKey, apiSecret := getTestCredentials(t)
	api := NewAPI(apiKey, apiSecret)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Make several rapid requests to test rate limiting behavior
	params := map[string]interface{}{
		"user":   "rj",
		"period": "7day",
		"limit":  "1",
	}

	successCount := 0
	rateLimitCount := 0

	for i := 0; i < 10; i++ {
		_, err := api.GetTopAlbums(ctx, params)

		if err == nil {
			successCount++
		} else if apiErr, ok := err.(*APIError); ok && apiErr.Code == 29 {
			rateLimitCount++
			t.Logf("Hit rate limit on request %d", i+1)
			break // Stop when we hit rate limit
		} else if apiErr, ok := err.(*APIError); ok && (apiErr.Code == 6 || apiErr.Code == 26) {
			// User not found or API key issues - skip this test
			t.Skipf("API error: %v", err)
		} else {
			t.Errorf("Unexpected error on request %d: %v", i+1, err)
		}

		// Small delay between requests
		time.Sleep(100 * time.Millisecond)
	}

	t.Logf("Successful requests: %d, Rate limited: %d", successCount, rateLimitCount)

	if successCount == 0 && rateLimitCount == 0 {
		t.Error("No successful requests and no rate limiting detected - something is wrong")
	}
}
