package lastfm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Test helpers

func createTestServer(statusCode int, response interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)

		if response != nil {
			if responseStr, ok := response.(string); ok {
				w.Write([]byte(responseStr))
			} else {
				json.NewEncoder(w).Encode(response)
			}
		}
	}))
}

func createErrorTestServer(statusCode int, errorCode int, message string) *httptest.Server {
	errorResponse := map[string]interface{}{
		"error":   errorCode,
		"message": message,
	}
	return createTestServer(statusCode, errorResponse)
}

func TestNewAPI(t *testing.T) {
	apiKey := "test-api-key"
	apiSecret := "test-api-secret"

	api := NewAPI(apiKey, apiSecret)

	if api.apiKey != apiKey {
		t.Errorf("Expected apiKey %s, got %s", apiKey, api.apiKey)
	}

	if api.apiSecret != apiSecret {
		t.Errorf("Expected apiSecret %s, got %s", apiSecret, api.apiSecret)
	}

	if api.baseURL != "https://ws.audioscrobbler.com/2.0/" {
		t.Errorf("Expected baseURL https://ws.audioscrobbler.com/2.0/, got %s", api.baseURL)
	}

	if api.client == nil {
		t.Error("Expected HTTP client to be initialized")
	}

	if api.client.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", api.client.Timeout)
	}
}

func TestAPI_makeRequest(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		response      interface{}
		method        string
		params        map[string]interface{}
		expectedError bool
		errorContains string
	}{
		{
			name:          "successful request",
			statusCode:    200,
			response:      map[string]string{"status": "ok"},
			method:        "test.method",
			params:        map[string]interface{}{"user": "testuser"},
			expectedError: false,
		},
		{
			name:          "http error",
			statusCode:    500,
			method:        "test.method",
			params:        map[string]interface{}{},
			expectedError: true,
			errorContains: "HTTP error: 500",
		},
		{
			name:          "api error",
			statusCode:    200,
			response:      map[string]interface{}{"error": 6, "message": "User not found"},
			method:        "test.method",
			params:        map[string]interface{}{},
			expectedError: true,
			errorContains: "Last.fm API error 6: User not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := createTestServer(tt.statusCode, tt.response)
			defer server.Close()

			api := NewAPI("test-key", "test-secret")
			api.baseURL = server.URL + "/"

			ctx := context.Background()
			body, err := api.makeRequest(ctx, tt.method, tt.params)

			if tt.expectedError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if body == nil {
					t.Error("Expected response body but got nil")
				}
			}
		})
	}
}

func TestAPI_makeRequest_contextCancellation(t *testing.T) {
	// Create a server that responds slowly
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return // Client cancelled
		case <-time.After(5 * time.Second):
			w.WriteHeader(200)
		}
	}))
	defer server.Close()

	api := NewAPI("test-key", "test-secret")
	api.baseURL = server.URL + "/"

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := api.makeRequest(ctx, "test.method", map[string]interface{}{})

	if err == nil {
		t.Error("Expected context cancellation error but got none")
	}

	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Expected context deadline exceeded error, got: %v", err)
	}
}

func TestAPI_GetTopAlbums(t *testing.T) {
	mockResponse := TopAlbumsResponse{
		TopAlbums: struct {
			Albums []Album `json:"album"`
		}{
			Albums: []Album{
				{
					Name:   "Test Album",
					Artist: Artist{Name: "Test Artist"},
					URL:    "http://test.com",
					Images: []Image{
						{Size: "large", URL: "http://test.com/image.jpg"},
					},
				},
			},
		},
	}

	server := createTestServer(200, mockResponse)
	defer server.Close()

	api := NewAPI("test-key", "test-secret")
	api.baseURL = server.URL + "/"

	ctx := context.Background()
	params := map[string]interface{}{
		"user":   "testuser",
		"period": "7day",
		"limit":  "10",
	}

	result, err := api.GetTopAlbums(ctx, params)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result but got nil")
	}

	if len(result.TopAlbums.Albums) != 1 {
		t.Errorf("Expected 1 album, got %d", len(result.TopAlbums.Albums))
	}

	album := result.TopAlbums.Albums[0]
	if album.Name != "Test Album" {
		t.Errorf("Expected album name 'Test Album', got %s", album.Name)
	}

	if album.Artist.Name != "Test Artist" {
		t.Errorf("Expected artist name 'Test Artist', got %s", album.Artist.Name)
	}
}

func TestAPI_GetTopArtists(t *testing.T) {
	mockResponse := TopArtistsResponse{
		TopArtists: struct {
			Artists []Artist `json:"artist"`
		}{
			Artists: []Artist{
				{
					Name: "Test Artist",
					URL:  "http://test.com",
				},
			},
		},
	}

	server := createTestServer(200, mockResponse)
	defer server.Close()

	api := NewAPI("test-key", "test-secret")
	api.baseURL = server.URL + "/"

	ctx := context.Background()
	params := map[string]interface{}{
		"user":   "testuser",
		"period": "7day",
		"limit":  "10",
	}

	result, err := api.GetTopArtists(ctx, params)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result but got nil")
	}

	if len(result.TopArtists.Artists) != 1 {
		t.Errorf("Expected 1 artist, got %d", len(result.TopArtists.Artists))
	}

	artist := result.TopArtists.Artists[0]
	if artist.Name != "Test Artist" {
		t.Errorf("Expected artist name 'Test Artist', got %s", artist.Name)
	}
}

func TestAPI_GetTopTracks(t *testing.T) {
	mockResponse := TopTracksResponse{
		TopTracks: struct {
			Tracks []Track `json:"track"`
		}{
			Tracks: []Track{
				{
					Name:   "Test Track",
					Artist: Artist{Name: "Test Artist"},
					URL:    "http://test.com",
				},
			},
		},
	}

	server := createTestServer(200, mockResponse)
	defer server.Close()

	api := NewAPI("test-key", "test-secret")
	api.baseURL = server.URL + "/"

	ctx := context.Background()
	params := map[string]interface{}{
		"user":   "testuser",
		"period": "7day",
		"limit":  "10",
	}

	result, err := api.GetTopTracks(ctx, params)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result but got nil")
	}

	if len(result.TopTracks.Tracks) != 1 {
		t.Errorf("Expected 1 track, got %d", len(result.TopTracks.Tracks))
	}

	track := result.TopTracks.Tracks[0]
	if track.Name != "Test Track" {
		t.Errorf("Expected track name 'Test Track', got %s", track.Name)
	}

	if track.Artist.Name != "Test Artist" {
		t.Errorf("Expected artist name 'Test Artist', got %s", track.Artist.Name)
	}
}

func TestAPI_GetRecentTracks(t *testing.T) {
	mockResponse := RecentTracksResponse{
		RecentTracks: struct {
			Tracks []Track `json:"track"`
		}{
			Tracks: []Track{
				{
					Name:    "Test Track",
					Artist:  Artist{Name: "Test Artist"},
					URL:     "http://test.com",
					Attribs: TrackAttribs{NowPlaying: "true"},
				},
			},
		},
	}

	server := createTestServer(200, mockResponse)
	defer server.Close()

	api := NewAPI("test-key", "test-secret")
	api.baseURL = server.URL + "/"

	ctx := context.Background()
	params := map[string]interface{}{
		"user":  "testuser",
		"limit": "10",
	}

	result, err := api.GetRecentTracks(ctx, params)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result but got nil")
	}

	if len(result.RecentTracks.Tracks) != 1 {
		t.Errorf("Expected 1 track, got %d", len(result.RecentTracks.Tracks))
	}

	track := result.RecentTracks.Tracks[0]
	if track.Name != "Test Track" {
		t.Errorf("Expected track name 'Test Track', got %s", track.Name)
	}

	if !track.IsNowPlaying() {
		t.Errorf("Expected track to be now playing")
	}
}

func TestAPI_GetWeeklyArtistChart(t *testing.T) {
	mockResponse := WeeklyArtistChartResponse{
		WeeklyArtistChart: struct {
			Artists []struct {
				Name      string `json:"name"`
				PlayCount string `json:"playcount"`
			} `json:"artist"`
		}{
			Artists: []struct {
				Name      string `json:"name"`
				PlayCount string `json:"playcount"`
			}{
				{
					Name:      "Test Artist",
					PlayCount: "42",
				},
			},
		},
	}

	server := createTestServer(200, mockResponse)
	defer server.Close()

	api := NewAPI("test-key", "test-secret")
	api.baseURL = server.URL + "/"

	ctx := context.Background()
	params := map[string]interface{}{
		"user": "testuser",
		"from": "1234567890",
		"to":   "1234567900",
	}

	result, err := api.GetWeeklyArtistChart(ctx, params)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result but got nil")
	}

	if len(result.WeeklyArtistChart.Artists) != 1 {
		t.Errorf("Expected 1 artist, got %d", len(result.WeeklyArtistChart.Artists))
	}

	artist := result.WeeklyArtistChart.Artists[0]
	if artist.Name != "Test Artist" {
		t.Errorf("Expected artist name 'Test Artist', got %s", artist.Name)
	}

	if artist.PlayCount != "42" {
		t.Errorf("Expected play count '42', got %s", artist.PlayCount)
	}
}

func TestAPI_GetArtistInfo(t *testing.T) {
	mockResponse := ArtistInfoResponse{
		Artist: ArtistInfo{
			Name: "Test Artist",
			URL:  "http://test.com",
			Stats: struct {
				UserPlays string `json:"userplaycount"`
			}{
				UserPlays: "100",
			},
		},
	}

	server := createTestServer(200, mockResponse)
	defer server.Close()

	api := NewAPI("test-key", "test-secret")
	api.baseURL = server.URL + "/"

	ctx := context.Background()
	params := map[string]interface{}{
		"artist":   "Test Artist",
		"username": "testuser",
	}

	result, err := api.GetArtistInfo(ctx, params)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result but got nil")
	}

	if result.Artist.Name != "Test Artist" {
		t.Errorf("Expected artist name 'Test Artist', got %s", result.Artist.Name)
	}

	if result.Artist.Stats.UserPlays != "100" {
		t.Errorf("Expected user plays '100', got %s", result.Artist.Stats.UserPlays)
	}
}

func TestAPI_GetAlbumInfo(t *testing.T) {
	mockResponse := AlbumInfoResponse{
		Album: AlbumInfo{
			Name:          "Test Album",
			Artist:        "Test Artist",
			URL:           "http://test.com",
			UserPlayCount: "50",
		},
	}

	server := createTestServer(200, mockResponse)
	defer server.Close()

	api := NewAPI("test-key", "test-secret")
	api.baseURL = server.URL + "/"

	ctx := context.Background()
	params := map[string]interface{}{
		"artist":   "Test Artist",
		"album":    "Test Album",
		"username": "testuser",
	}

	result, err := api.GetAlbumInfo(ctx, params)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result but got nil")
	}

	if result.Album.Name != "Test Album" {
		t.Errorf("Expected album name 'Test Album', got %s", result.Album.Name)
	}

	if result.Album.Artist != "Test Artist" {
		t.Errorf("Expected artist name 'Test Artist', got %s", result.Album.Artist)
	}

	if result.Album.UserPlayCount != "50" {
		t.Errorf("Expected user play count '50', got %s", result.Album.UserPlayCount)
	}
}

func TestAPI_GetTrackInfo(t *testing.T) {
	mockResponse := TrackInfoResponse{
		Track: TrackInfo{
			Name:          "Test Track",
			Artist:        Artist{Name: "Test Artist"},
			URL:           "http://test.com",
			UserPlayCount: "25",
		},
	}

	server := createTestServer(200, mockResponse)
	defer server.Close()

	api := NewAPI("test-key", "test-secret")
	api.baseURL = server.URL + "/"

	ctx := context.Background()
	params := map[string]interface{}{
		"artist":   "Test Artist",
		"track":    "Test Track",
		"username": "testuser",
	}

	result, err := api.GetTrackInfo(ctx, params)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result but got nil")
	}

	if result.Track.Name != "Test Track" {
		t.Errorf("Expected track name 'Test Track', got %s", result.Track.Name)
	}

	if result.Track.Artist.Name != "Test Artist" {
		t.Errorf("Expected artist name 'Test Artist', got %s", result.Track.Artist.Name)
	}

	if result.Track.UserPlayCount != "25" {
		t.Errorf("Expected user play count '25', got %s", result.Track.UserPlayCount)
	}
}

func TestAPIError_Error(t *testing.T) {
	apiError := &APIError{
		Code:    6,
		Message: "User not found",
	}

	expected := "Last.fm API error 6: User not found"
	if apiError.Error() != expected {
		t.Errorf("Expected error message %q, got %q", expected, apiError.Error())
	}
}

func TestTrack_JsonUnmarshal_RealWorldExample(t *testing.T) {
	// Test with real-world JSON examples from Last.fm API
	tests := []struct {
		name               string
		jsonData           string
		expectedNowPlaying bool
	}{
		{
			name: "now playing track",
			jsonData: `{
				"name": "Test Track",
				"artist": {"name": "Test Artist"},
				"@attr": {"nowplaying": "true"}
			}`,
			expectedNowPlaying: true,
		},
		{
			name: "regular track with date",
			jsonData: `{
				"name": "Regular Track",
				"artist": {"name": "Regular Artist"},
				"date": {"uts": "1234567890", "#text": "01 Jan 2009"}
			}`,
			expectedNowPlaying: false,
		},
		{
			name: "track with empty attr",
			jsonData: `{
				"name": "Empty Attr Track",
				"artist": {"name": "Empty Artist"},
				"@attr": {}
			}`,
			expectedNowPlaying: false,
		},
		{
			name: "track without attr field",
			jsonData: `{
				"name": "No Attr Track",
				"artist": {"name": "No Attr Artist"}
			}`,
			expectedNowPlaying: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var track Track
			err := json.Unmarshal([]byte(tt.jsonData), &track)
			if err != nil {
				t.Fatalf("Failed to unmarshal JSON: %v", err)
			}

			if track.IsNowPlaying() != tt.expectedNowPlaying {
				t.Errorf("Expected IsNowPlaying() to be %v, got %v", tt.expectedNowPlaying, track.IsNowPlaying())
			}
		})
	}
}

func TestAlbum_UnmarshalJSON_RobustHandling(t *testing.T) {
	// Test the improved Album UnmarshalJSON method
	tests := []struct {
		name           string
		jsonData       string
		expectedName   string
		expectedArtist string
		expectError    bool
	}{
		{
			name:           "artist as object",
			jsonData:       `{"name": "Test Album", "artist": {"name": "Test Artist", "url": "http://test.com"}}`,
			expectedName:   "Test Album",
			expectedArtist: "Test Artist",
			expectError:    false,
		},
		{
			name:           "artist as string",
			jsonData:       `{"name": "Test Album", "artist": "String Artist"}`,
			expectedName:   "Test Album",
			expectedArtist: "String Artist",
			expectError:    false,
		},
		{
			name:           "artist as null",
			jsonData:       `{"name": "Test Album", "artist": null}`,
			expectedName:   "Test Album",
			expectedArtist: "",
			expectError:    false,
		},
		{
			name:           "missing artist field",
			jsonData:       `{"name": "Test Album"}`,
			expectedName:   "Test Album",
			expectedArtist: "",
			expectError:    false,
		},
		{
			name:           "artist as number (edge case)",
			jsonData:       `{"name": "Test Album", "artist": 123}`,
			expectedName:   "Test Album",
			expectedArtist: "123",
			expectError:    false,
		},
		{
			name:        "invalid json",
			jsonData:    `{"name": "Test Album", "artist": }`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var album Album
			err := json.Unmarshal([]byte(tt.jsonData), &album)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if album.Name != tt.expectedName {
				t.Errorf("Expected name %q, got %q", tt.expectedName, album.Name)
			}

			if album.Artist.Name != tt.expectedArtist {
				t.Errorf("Expected artist name %q, got %q", tt.expectedArtist, album.Artist.Name)
			}
		})
	}
}

func TestAPI_makeRequest_ImprovedErrorHandling(t *testing.T) {
	// Test the improved makeRequest method error handling
	tests := []struct {
		name          string
		response      string
		statusCode    int
		expectedError bool
		errorContains string
	}{
		{
			name:          "valid response without error field",
			response:      `{"topalbums": {"album": []}}`,
			statusCode:    200,
			expectedError: false,
		},
		{
			name:          "api error response",
			response:      `{"error": 6, "message": "User not found"}`,
			statusCode:    200,
			expectedError: true,
			errorContains: "Last.fm API error 6: User not found",
		},
		{
			name:          "invalid json response",
			response:      `{invalid json`,
			statusCode:    200,
			expectedError: true,
			errorContains: "invalid JSON response",
		},
		{
			name:          "empty response",
			response:      ``,
			statusCode:    200,
			expectedError: true,
			errorContains: "invalid JSON response",
		},
		{
			name:          "response with error field set to 0 (no error)",
			response:      `{"error": 0, "topalbums": {"album": []}}`,
			statusCode:    200,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.response))
			}))
			defer server.Close()

			api := NewAPI("test-key", "test-secret")
			api.baseURL = server.URL + "/"

			ctx := context.Background()
			_, err := api.makeRequest(ctx, "test.method", map[string]interface{}{})

			if tt.expectedError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}
