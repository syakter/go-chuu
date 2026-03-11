package lastfm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// Error scenario tests

func TestAPI_ErrorResponses(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		response      interface{}
		expectedError bool
		errorType     string
		errorCode     int
	}{
		{
			name:          "User not found",
			statusCode:    200,
			response:      map[string]interface{}{"error": 6, "message": "User not found"},
			expectedError: true,
			errorType:     "api",
			errorCode:     6,
		},
		{
			name:          "Invalid API key",
			statusCode:    200,
			response:      map[string]interface{}{"error": 26, "message": "Suspended API key"},
			expectedError: true,
			errorType:     "api",
			errorCode:     26,
		},
		{
			name:          "Rate limit exceeded",
			statusCode:    200,
			response:      map[string]interface{}{"error": 29, "message": "Rate limit exceeded"},
			expectedError: true,
			errorType:     "api",
			errorCode:     29,
		},
		{
			name:          "HTTP 500 error",
			statusCode:    500,
			response:      nil,
			expectedError: true,
			errorType:     "http",
		},
		{
			name:          "HTTP 403 error",
			statusCode:    403,
			response:      nil,
			expectedError: true,
			errorType:     "http",
		},
		{
			name:          "Invalid JSON response",
			statusCode:    200,
			response:      "invalid json{\"incomplete",
			expectedError: true,
			errorType:     "json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server *httptest.Server
			if tt.name == "Invalid JSON response" {
				// Create a server that returns invalid JSON
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tt.statusCode)
					w.Write([]byte("invalid json{\"incomplete"))
				}))
			} else {
				server = createTestServer(tt.statusCode, tt.response)
			}
			defer server.Close()

			api := NewAPI("test-key", "test-secret")
			api.baseURL = server.URL + "/"

			ctx := context.Background()
			params := map[string]interface{}{"user": "testuser"}

			var err error
			if tt.name == "Invalid JSON response" {
				// Call an actual API method that will try to parse JSON
				_, err = api.GetTopAlbums(ctx, params)
			} else {
				_, err = api.makeRequest(ctx, "test.method", params)
			}

			if !tt.expectedError {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				return
			}

			if err == nil {
				t.Error("Expected error but got none")
				return
			}

			switch tt.errorType {
			case "api":
				apiErr, ok := err.(*APIError)
				if !ok {
					t.Errorf("Expected APIError but got %T: %v", err, err)
					return
				}
				if apiErr.Code != tt.errorCode {
					t.Errorf("Expected error code %d, got %d", tt.errorCode, apiErr.Code)
				}
			case "http":
				if !strings.Contains(err.Error(), "HTTP error") {
					t.Errorf("Expected HTTP error, got: %v", err)
				}
			case "json":
				// JSON errors can be various types, just check it's not an APIError
				if _, ok := err.(*APIError); ok {
					t.Errorf("Expected JSON parsing error but got APIError: %v", err)
				}
			}
		})
	}
}

func TestClient_ErrorPropagation(t *testing.T) {
	// Test that errors from the API layer are properly propagated through the Client layer
	client := createTestClient()

	// Create a mock API that returns errors
	errorAPI := &errorMockAPI{}
	client.api = errorAPI

	ctx := context.Background()

	tests := []struct {
		name        string
		fn          func() error
		expectError bool // Some methods are resilient and don't propagate individual API errors
	}{
		{"GetArtistScrobbles", func() error {
			_, err := client.GetArtistScrobbles(ctx, "Test Artist")
			return err
		}, false}, // Resilient - logs errors but doesn't fail
		{"GetAlbumScrobbles", func() error {
			_, err := client.GetAlbumScrobbles(ctx, "Test Artist", "Test Album")
			return err
		}, false}, // Resilient - logs errors but doesn't fail
		{"GetTrackScrobbles", func() error {
			_, err := client.GetTrackScrobbles(ctx, "Test Artist", "Test Track")
			return err
		}, false}, // Resilient - logs errors but doesn't fail
		{"GetUserTopAlbums", func() error {
			_, err := client.GetUserTopAlbums(ctx, "testuser", "7day", 10)
			return err
		}, true}, // Not resilient - propagates API errors
		{"GetUserTopArtists", func() error {
			_, err := client.GetUserTopArtists(ctx, "testuser", "7day", 10)
			return err
		}, true}, // Not resilient - propagates API errors
		{"GetUserTopTracks", func() error {
			_, err := client.GetUserTopTracks(ctx, "testuser", "7day", 10)
			return err
		}, true}, // Not resilient - propagates API errors
		{"GetUserRecentTracks", func() error {
			_, err := client.GetUserRecentTracks(ctx, "testuser", 10)
			return err
		}, true}, // Not resilient - propagates API errors
		{"GetWeeklyLeaderboard", func() error {
			_, err := client.GetWeeklyLeaderboard(ctx)
			return err
		}, false}, // Resilient - logs errors but doesn't fail
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}

				// Errors should be wrapped, so check for API error information
				if !strings.Contains(err.Error(), "Last.fm API error") && !strings.Contains(err.Error(), "failed to") {
					t.Errorf("Expected wrapped API error, got: %v", err)
				}
			} else {
				// These methods are resilient and should not propagate individual API errors
				if err != nil {
					t.Errorf("Expected resilient method to not fail, but got error: %v", err)
				}
			}
		})
	}
}

func TestClient_ContextCancellationScenarios(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
		delay   time.Duration
	}{
		{"Immediate cancellation", 0, 100 * time.Millisecond},
		{"Short timeout", 10 * time.Millisecond, 100 * time.Millisecond},
		{"Medium timeout", 50 * time.Millisecond, 100 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a server that responds slowly
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(tt.delay)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			}))
			defer server.Close()

			api := NewAPI("test-key", "test-secret")
			api.baseURL = server.URL + "/"

			var ctx context.Context
			var cancel context.CancelFunc

			if tt.timeout == 0 {
				ctx, cancel = context.WithCancel(context.Background())
				cancel() // Cancel immediately
			} else {
				ctx, cancel = context.WithTimeout(context.Background(), tt.timeout)
				defer cancel()
			}

			_, err := api.makeRequest(ctx, "test.method", map[string]interface{}{})

			if err == nil {
				t.Error("Expected context cancellation error but got none")
				return
			}

			if !strings.Contains(err.Error(), "context") {
				t.Errorf("Expected context-related error, got: %v", err)
			}
		})
	}
}

func TestClient_ConcurrencyLimiting(t *testing.T) {
	// Test that the semaphore properly limits concurrent requests
	client := createTestClientWithUsers([]string{"user1", "user2", "user3", "user4", "user5"})
	client.maxConcurrentRequests = 2
	client.semaphore = make(chan struct{}, 2)

	// Create a slow mock API to test concurrency
	slowAPI := &slowMockAPI{
		calls:     make(map[string]int),
		delay:     100 * time.Millisecond,
		active:    0,
		maxActive: 0,
	}
	client.api = slowAPI

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.GetArtistScrobbles(ctx, "Test Artist")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Should have made 5 API calls (one per user)
	if slowAPI.calls["GetArtistInfo"] != 5 {
		t.Errorf("Expected 5 API calls, got %d", slowAPI.calls["GetArtistInfo"])
	}

	// Should not have exceeded our concurrency limit
	if slowAPI.maxActive > 2 {
		t.Errorf("Expected max concurrent requests to be 2, got %d", slowAPI.maxActive)
	}
}

type slowMockAPI struct {
	calls     map[string]int
	delay     time.Duration
	active    int
	maxActive int
	mu        sync.Mutex
}

func (m *slowMockAPI) trackCall(method string) {
	m.mu.Lock()
	m.calls[method]++
	m.active++
	if m.active > m.maxActive {
		m.maxActive = m.active
	}
	m.mu.Unlock()

	time.Sleep(m.delay)

	m.mu.Lock()
	m.active--
	m.mu.Unlock()
}

func (m *slowMockAPI) GetArtistInfo(ctx context.Context, params map[string]interface{}) (*ArtistInfoResponse, error) {
	m.trackCall("GetArtistInfo")
	return &ArtistInfoResponse{
		Artist: ArtistInfo{
			Name: "Mock Artist",
			Stats: struct {
				UserPlays string `json:"userplaycount"`
			}{
				UserPlays: "100",
			},
		},
	}, nil
}

// Implement other required methods for slowMockAPI
func (m *slowMockAPI) GetTopAlbums(ctx context.Context, params map[string]interface{}) (*TopAlbumsResponse, error) {
	m.trackCall("GetTopAlbums")
	return &TopAlbumsResponse{}, nil
}

func (m *slowMockAPI) GetTopArtists(ctx context.Context, params map[string]interface{}) (*TopArtistsResponse, error) {
	m.trackCall("GetTopArtists")
	return &TopArtistsResponse{}, nil
}

func (m *slowMockAPI) GetTopTracks(ctx context.Context, params map[string]interface{}) (*TopTracksResponse, error) {
	m.trackCall("GetTopTracks")
	return &TopTracksResponse{}, nil
}

func (m *slowMockAPI) GetRecentTracks(ctx context.Context, params map[string]interface{}) (*RecentTracksResponse, error) {
	m.trackCall("GetRecentTracks")
	return &RecentTracksResponse{}, nil
}

func (m *slowMockAPI) GetWeeklyArtistChart(ctx context.Context, params map[string]interface{}) (*WeeklyArtistChartResponse, error) {
	m.trackCall("GetWeeklyArtistChart")
	return &WeeklyArtistChartResponse{}, nil
}

func (m *slowMockAPI) GetAlbumInfo(ctx context.Context, params map[string]interface{}) (*AlbumInfoResponse, error) {
	m.trackCall("GetAlbumInfo")
	return &AlbumInfoResponse{}, nil
}

func (m *slowMockAPI) GetTrackInfo(ctx context.Context, params map[string]interface{}) (*TrackInfoResponse, error) {
	m.trackCall("GetTrackInfo")
	return &TrackInfoResponse{}, nil
}

func (m *slowMockAPI) GetArtistTopAlbums(ctx context.Context, params map[string]interface{}) (*TopAlbumsResponse, error) {
	m.trackCall("GetArtistTopAlbums")
	return &TopAlbumsResponse{}, nil
}

func (m *slowMockAPI) GetArtistTopTracks(ctx context.Context, params map[string]interface{}) (*TopTracksResponse, error) {
	m.trackCall("GetArtistTopTracks")
	return &TopTracksResponse{}, nil
}

func TestClient_EmptyResponseHandling(t *testing.T) {
	// Test how the client handles empty or minimal responses
	tests := []struct {
		name     string
		response interface{}
		method   string
	}{
		{
			name: "Empty albums response",
			response: TopAlbumsResponse{
				TopAlbums: struct {
					Albums []Album `json:"album"`
				}{
					Albums: []Album{},
				},
			},
			method: "user.getTopAlbums",
		},
		{
			name: "Empty artists response",
			response: TopArtistsResponse{
				TopArtists: struct {
					Artists []Artist `json:"artist"`
				}{
					Artists: []Artist{},
				},
			},
			method: "user.getTopArtists",
		},
		{
			name: "Empty tracks response",
			response: RecentTracksResponse{
				RecentTracks: struct {
					Tracks []Track `json:"track"`
				}{
					Tracks: []Track{},
				},
			},
			method: "user.getRecentTracks",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := createTestServer(200, tt.response)
			defer server.Close()

			api := NewAPI("test-key", "test-secret")
			api.baseURL = server.URL + "/"

			ctx := context.Background()
			params := map[string]interface{}{"user": "testuser"}

			// Test each API method with empty responses
			switch tt.method {
			case "user.getTopAlbums":
				result, err := api.GetTopAlbums(ctx, params)
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result == nil {
					t.Error("Expected result but got nil")
				}
				if len(result.TopAlbums.Albums) != 0 {
					t.Errorf("Expected empty albums slice, got %d items", len(result.TopAlbums.Albums))
				}
			case "user.getTopArtists":
				result, err := api.GetTopArtists(ctx, params)
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result == nil {
					t.Error("Expected result but got nil")
				}
				if len(result.TopArtists.Artists) != 0 {
					t.Errorf("Expected empty artists slice, got %d items", len(result.TopArtists.Artists))
				}
			case "user.getRecentTracks":
				result, err := api.GetRecentTracks(ctx, params)
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result == nil {
					t.Error("Expected result but got nil")
				}
				if len(result.RecentTracks.Tracks) != 0 {
					t.Errorf("Expected empty tracks slice, got %d items", len(result.RecentTracks.Tracks))
				}
			}
		})
	}
}

func TestClient_EdgeCases(t *testing.T) {
	client := createTestClient()
	ctx := context.Background()

	tests := []struct {
		name string
		fn   func() error
	}{
		{"Empty artist name", func() error {
			_, err := client.GetArtistScrobbles(ctx, "")
			return err
		}},
		{"Empty album name", func() error {
			_, err := client.GetAlbumScrobbles(ctx, "Artist", "")
			return err
		}},
		{"Empty track name", func() error {
			_, err := client.GetTrackScrobbles(ctx, "Artist", "")
			return err
		}},
		{"Empty username", func() error {
			_, err := client.GetUserTopAlbums(ctx, "", "7day", 10)
			return err
		}},
		{"Zero limit", func() error {
			_, err := client.GetUserTopAlbums(ctx, "user", "7day", 0)
			return err
		}},
		{"Negative limit", func() error {
			_, err := client.GetUserTopAlbums(ctx, "user", "7day", -1)
			return err
		}},
		{"Very large limit", func() error {
			_, err := client.GetUserTopAlbums(ctx, "user", "7day", 10000)
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			// These should not panic, though they may return errors
			// The specific behavior depends on the Last.fm API
			t.Logf("Result for %s: %v", tt.name, err)
		})
	}
}

func TestClient_SpecialCharacters(t *testing.T) {
	client := createTestClient()
	ctx := context.Background()

	tests := []struct {
		name   string
		artist string
	}{
		{"Artist with ampersand entity", "Artist &amp; Band"},
		{"Artist with unicode", "Sigur Rós"},
		{"Artist with quotes", `Artist "The" Band`},
		{"Artist with apostrophes", "Artist's Band"},
		{"Artist with slashes", "AC/DC"},
		{"Artist with spaces", "   Artist   With   Spaces   "},
		{"Artist with numbers", "Artist 123"},
		{"Artist with special chars", "Artist!@#$%^&*()_+-="},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// These should not panic
			_, err := client.GetArtistScrobbles(ctx, tt.artist)
			t.Logf("Result for artist %q: %v", tt.artist, err)
		})
	}
}

// errorMockAPI is a mock API that returns errors for testing
type errorMockAPI struct{}

func (e *errorMockAPI) GetTopAlbums(ctx context.Context, params map[string]interface{}) (*TopAlbumsResponse, error) {
	return nil, &APIError{Code: 6, Message: "User not found"}
}

func (e *errorMockAPI) GetTopArtists(ctx context.Context, params map[string]interface{}) (*TopArtistsResponse, error) {
	return nil, &APIError{Code: 6, Message: "User not found"}
}

func (e *errorMockAPI) GetTopTracks(ctx context.Context, params map[string]interface{}) (*TopTracksResponse, error) {
	return nil, &APIError{Code: 6, Message: "User not found"}
}

func (e *errorMockAPI) GetRecentTracks(ctx context.Context, params map[string]interface{}) (*RecentTracksResponse, error) {
	return nil, &APIError{Code: 6, Message: "User not found"}
}

func (e *errorMockAPI) GetWeeklyArtistChart(ctx context.Context, params map[string]interface{}) (*WeeklyArtistChartResponse, error) {
	return nil, &APIError{Code: 6, Message: "User not found"}
}

func (e *errorMockAPI) GetArtistInfo(ctx context.Context, params map[string]interface{}) (*ArtistInfoResponse, error) {
	return nil, &APIError{Code: 6, Message: "User not found"}
}

func (e *errorMockAPI) GetAlbumInfo(ctx context.Context, params map[string]interface{}) (*AlbumInfoResponse, error) {
	return nil, &APIError{Code: 6, Message: "User not found"}
}

func (e *errorMockAPI) GetTrackInfo(ctx context.Context, params map[string]interface{}) (*TrackInfoResponse, error) {
	return nil, &APIError{Code: 6, Message: "User not found"}
}

func (e *errorMockAPI) GetArtistTopAlbums(ctx context.Context, params map[string]interface{}) (*TopAlbumsResponse, error) {
	return nil, &APIError{Code: 6, Message: "User not found"}
}

func (e *errorMockAPI) GetArtistTopTracks(ctx context.Context, params map[string]interface{}) (*TopTracksResponse, error) {
	return nil, &APIError{Code: 6, Message: "User not found"}
}
