package lastfm

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetRecentTracks(t *testing.T) {
	// Create mock server with test response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method
		if r.URL.Query().Get("method") != "user.getRecentTracks" {
			t.Errorf("Expected method 'user.getRecentTracks', got %s", r.URL.Query().Get("method"))
		}

		// Return mock response
		mockResponse := `{
			"recenttracks": {
				"track": [
					{
						"artist": {
							"#text": "Test Artist"
						},
						"name": "Test Track"
					},
					{
						"artist": {
							"#text": "Test Artist 2"
						},
						"name": "Test Track 2"
					}
				]
			}
		}`
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	// Create API client with test server
	api := New("test_key", "test_secret")
	api.BaseURL = server.URL + "/"

	// Test GetRecentTracks
	result, err := api.User.GetRecentTracks(P{"user": "testuser", "limit": 2})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify results
	if len(result.Tracks) != 2 {
		t.Errorf("Expected 2 tracks, got %d", len(result.Tracks))
	}

	// Check first track
	if result.Tracks[0].Artist.Name != "Test Artist" {
		t.Errorf("Expected artist 'Test Artist', got %s", result.Tracks[0].Artist.Name)
	}
	if result.Tracks[0].Name != "Test Track" {
		t.Errorf("Expected track 'Test Track', got %s", result.Tracks[0].Name)
	}
}

func TestGetTopArtists(t *testing.T) {
	// Create mock server with test response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method
		if r.URL.Query().Get("method") != "user.getTopArtists" {
			t.Errorf("Expected method 'user.getTopArtists', got %s", r.URL.Query().Get("method"))
		}

		// Return mock response
		mockResponse := `{
			"topartists": {
				"artist": [
					{
						"name": "Top Artist 1",
						"playcount": "100",
						"listeners": "50",
						"mbid": "123",
						"url": "http://test.com",
						"streamable": "1"
					},
					{
						"name": "Top Artist 2",
						"playcount": "80",
						"listeners": "40",
						"mbid": "456",
						"url": "http://test2.com",
						"streamable": "1"
					}
				]
			}
		}`
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	// Create API client with test server
	api := New("test_key", "test_secret")
	api.BaseURL = server.URL + "/"

	// Test GetTopArtists
	result, err := api.User.GetTopArtists(P{"user": "testuser", "period": "7day"})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify results
	if len(result.Artists) != 2 {
		t.Errorf("Expected 2 artists, got %d", len(result.Artists))
	}

	// Check first artist
	if result.Artists[0].Name != "Top Artist 1" {
		t.Errorf("Expected artist 'Top Artist 1', got %s", result.Artists[0].Name)
	}
}
