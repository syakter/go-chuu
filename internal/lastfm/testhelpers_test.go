package lastfm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestHelpers provides common utilities for testing the lastfm package

// Common test data structures
var (
	TestAlbum = Album{
		Name:   "Test Album",
		Artist: Artist{Name: "Test Artist"},
		URL:    "http://test.com/album",
		Images: []Image{
			{Size: "small", URL: "http://test.com/small.jpg"},
			{Size: "medium", URL: "http://test.com/medium.jpg"},
			{Size: "large", URL: "http://test.com/large.jpg"},
			{Size: "extralarge", URL: "http://test.com/extralarge.jpg"},
		},
	}

	TestArtist = Artist{
		Name: "Test Artist",
		URL:  "http://test.com/artist",
	}

	TestTrack = Track{
		Name:   "Test Track",
		Artist: TestArtist,
		Album:  TestAlbum,
		URL:    "http://test.com/track",
		Images: []Image{
			{Size: "large", URL: "http://test.com/track_large.jpg"},
		},
		NowPlaying: "false",
		Date: struct {
			UTS  string `json:"uts"`
			Text string `json:"#text"`
		}{
			UTS:  "1234567890",
			Text: "01 Jan 2009, 00:31",
		},
	}

	TestNowPlayingTrack = Track{
		Name:       "Now Playing Track",
		Artist:     TestArtist,
		Album:      TestAlbum,
		URL:        "http://test.com/nowplaying",
		NowPlaying: "true",
	}
)

// MockResponseBuilder helps create structured API responses
type MockResponseBuilder struct {
	data interface{}
}

func NewMockResponse() *MockResponseBuilder {
	return &MockResponseBuilder{}
}

func (b *MockResponseBuilder) WithTopAlbums(albums []Album) *MockResponseBuilder {
	b.data = TopAlbumsResponse{
		TopAlbums: struct {
			Albums []Album `json:"album"`
		}{
			Albums: albums,
		},
	}
	return b
}

func (b *MockResponseBuilder) WithTopArtists(artists []Artist) *MockResponseBuilder {
	b.data = TopArtistsResponse{
		TopArtists: struct {
			Artists []Artist `json:"artist"`
		}{
			Artists: artists,
		},
	}
	return b
}

func (b *MockResponseBuilder) WithTopTracks(tracks []Track) *MockResponseBuilder {
	b.data = TopTracksResponse{
		TopTracks: struct {
			Tracks []Track `json:"track"`
		}{
			Tracks: tracks,
		},
	}
	return b
}

func (b *MockResponseBuilder) WithRecentTracks(tracks []Track) *MockResponseBuilder {
	b.data = RecentTracksResponse{
		RecentTracks: struct {
			Tracks []Track `json:"track"`
		}{
			Tracks: tracks,
		},
	}
	return b
}

func (b *MockResponseBuilder) WithArtistInfo(name, userPlays string) *MockResponseBuilder {
	b.data = ArtistInfoResponse{
		Artist: ArtistInfo{
			Name: name,
			URL:  "http://test.com/artist",
			Stats: struct {
				UserPlays string `json:"userplaycount"`
			}{
				UserPlays: userPlays,
			},
		},
	}
	return b
}

func (b *MockResponseBuilder) WithAlbumInfo(name, artist, userPlayCount string) *MockResponseBuilder {
	b.data = AlbumInfoResponse{
		Album: AlbumInfo{
			Name:          name,
			Artist:        artist,
			URL:           "http://test.com/album",
			UserPlayCount: userPlayCount,
		},
	}
	return b
}

func (b *MockResponseBuilder) WithTrackInfo(name, artist, userPlayCount string) *MockResponseBuilder {
	b.data = TrackInfoResponse{
		Track: TrackInfo{
			Name:          name,
			Artist:        Artist{Name: artist},
			URL:           "http://test.com/track",
			UserPlayCount: userPlayCount,
		},
	}
	return b
}

func (b *MockResponseBuilder) WithWeeklyChart(artists []struct{ Name, PlayCount string }) *MockResponseBuilder {
	chartArtists := make([]struct {
		Name      string `json:"name"`
		PlayCount string `json:"playcount"`
	}, len(artists))

	for i, artist := range artists {
		chartArtists[i] = struct {
			Name      string `json:"name"`
			PlayCount string `json:"playcount"`
		}{
			Name:      artist.Name,
			PlayCount: artist.PlayCount,
		}
	}

	b.data = WeeklyArtistChartResponse{
		WeeklyArtistChart: struct {
			Artists []struct {
				Name      string `json:"name"`
				PlayCount string `json:"playcount"`
			} `json:"artist"`
		}{
			Artists: chartArtists,
		},
	}
	return b
}

func (b *MockResponseBuilder) WithError(code int, message string) *MockResponseBuilder {
	b.data = map[string]interface{}{
		"error":   code,
		"message": message,
	}
	return b
}

func (b *MockResponseBuilder) Build() interface{} {
	return b.data
}

// MockServer creates an HTTP test server with configurable responses
type MockServer struct {
	server    *httptest.Server
	responses map[string]interface{}
	status    int
	delay     time.Duration
}

func NewMockServer() *MockServer {
	m := &MockServer{
		responses: make(map[string]interface{}),
		status:    200,
	}

	m.server = httptest.NewServer(http.HandlerFunc(m.handler))
	return m
}

func (m *MockServer) handler(w http.ResponseWriter, r *http.Request) {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}

	method := r.URL.Query().Get("method")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(m.status)

	if response, exists := m.responses[method]; exists {
		json.NewEncoder(w).Encode(response)
	} else {
		// Default response
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func (m *MockServer) SetResponse(method string, response interface{}) *MockServer {
	m.responses[method] = response
	return m
}

func (m *MockServer) SetStatus(status int) *MockServer {
	m.status = status
	return m
}

func (m *MockServer) SetDelay(delay time.Duration) *MockServer {
	m.delay = delay
	return m
}

func (m *MockServer) URL() string {
	return m.server.URL
}

func (m *MockServer) Close() {
	m.server.Close()
}

// Test assertion helpers

func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func AssertError(t *testing.T, err error, expectedMessage string) {
	t.Helper()
	if err == nil {
		t.Error("Expected error but got none")
		return
	}
	if expectedMessage != "" && err.Error() != expectedMessage {
		t.Errorf("Expected error message %q, got %q", expectedMessage, err.Error())
	}
}

func AssertAPIError(t *testing.T, err error, expectedCode int) {
	t.Helper()
	if err == nil {
		t.Error("Expected API error but got none")
		return
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Errorf("Expected APIError but got %T: %v", err, err)
		return
	}

	if apiErr.Code != expectedCode {
		t.Errorf("Expected API error code %d, got %d", expectedCode, apiErr.Code)
	}
}

func AssertEqual(t *testing.T, expected, actual interface{}) {
	t.Helper()
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

func AssertNotEmpty(t *testing.T, str string, fieldName string) {
	t.Helper()
	if str == "" {
		t.Errorf("Expected %s to be non-empty", fieldName)
	}
}

func AssertSliceLength(t *testing.T, slice interface{}, expectedLength int, sliceName string) {
	t.Helper()
	var length int
	switch s := slice.(type) {
	case []Album:
		length = len(s)
	case []Artist:
		length = len(s)
	case []Track:
		length = len(s)
	case []string:
		length = len(s)
	default:
		t.Errorf("Unsupported slice type for length assertion: %T", slice)
		return
	}

	if length != expectedLength {
		t.Errorf("Expected %s to have length %d, got %d", sliceName, expectedLength, length)
	}
}

// Benchmark helpers

func runBenchmarkHelper(b *testing.B, fn func()) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fn()
	}
}

// Test data generators

func GenerateTestAlbums(count int) []Album {
	albums := make([]Album, count)
	for i := 0; i < count; i++ {
		albums[i] = Album{
			Name:   fmt.Sprintf("Album %d", i+1),
			Artist: Artist{Name: fmt.Sprintf("Artist %d", i+1)},
			URL:    fmt.Sprintf("http://test.com/album%d", i+1),
			Images: []Image{
				{Size: "large", URL: fmt.Sprintf("http://test.com/album%d_large.jpg", i+1)},
			},
		}
	}
	return albums
}

func GenerateTestArtists(count int) []Artist {
	artists := make([]Artist, count)
	for i := 0; i < count; i++ {
		artists[i] = Artist{
			Name: fmt.Sprintf("Artist %d", i+1),
			URL:  fmt.Sprintf("http://test.com/artist%d", i+1),
		}
	}
	return artists
}

func GenerateTestTracks(count int) []Track {
	tracks := make([]Track, count)
	for i := 0; i < count; i++ {
		tracks[i] = Track{
			Name:       fmt.Sprintf("Track %d", i+1),
			Artist:     Artist{Name: fmt.Sprintf("Artist %d", i+1)},
			URL:        fmt.Sprintf("http://test.com/track%d", i+1),
			NowPlaying: "false",
		}
	}
	return tracks
}
