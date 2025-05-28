package lastfm

import (
	"net/http"
	"net/http/httptest"
)

// createTestServer creates a test server with the given response
func createTestServer(response string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
}

// createTestClient creates a test API client with the given server URL
func createTestClient(serverURL string) *Api {
	api := New("test_key", "test_secret")
	api.BaseURL = serverURL + "/"
	return api
}

// mockResponses contains sample JSON responses for testing
var mockResponses = struct {
	RecentTracks string
	TopArtists   string
}{
	RecentTracks: `{
		"recenttracks": {
			"track": [
				{
					"artist": {"#text": "Test Artist"},
					"name": "Test Track"
				}
			]
		}
	}`,
	TopArtists: `{
		"topartists": {
			"artist": [
				{
					"name": "Test Artist",
					"playcount": "100"
				}
			]
		}
	}`,
}
