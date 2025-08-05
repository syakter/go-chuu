package lastfm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// APIError represents a Last.fm API error
type APIError struct {
	Code    int    `json:"error"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("Last.fm API error %d: %s", e.Code, e.Message)
}

// API represents the Last.fm API client
type API struct {
	apiKey    string
	apiSecret string
	baseURL   string
	client    *http.Client
}

// NewAPI creates a new Last.fm API client
func NewAPI(apiKey, apiSecret string) *API {
	return &API{
		apiKey:    apiKey,
		apiSecret: apiSecret,
		baseURL:   "https://ws.audioscrobbler.com/2.0/",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// makeRequest makes a request to the Last.fm API
func (api *API) makeRequest(ctx context.Context, method string, params map[string]interface{}) ([]byte, error) {
	u, err := url.Parse(api.baseURL)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("method", method)
	q.Set("api_key", api.apiKey)
	q.Set("format", "json")

	for key, value := range params {
		q.Set(key, fmt.Sprintf("%v", value))
	}

	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := api.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Check for API errors in the response
	var errorResp APIError
	if err := json.Unmarshal(body, &errorResp); err == nil && errorResp.Code != 0 {
		return nil, &errorResp
	}

	return body, nil
}

// Image represents an image with different sizes
type Image struct {
	Size string `json:"size"`
	URL  string `json:"#text"`
}

// Artist represents an artist
type Artist struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Album represents an album
type Album struct {
	Name   string  `json:"name"`
	Artist Artist  `json:"artist"`
	URL    string  `json:"url"`
	Images []Image `json:"image"`
}

// Track represents a track
type Track struct {
	Name       string  `json:"name"`
	Artist     Artist  `json:"artist"`
	Album      Album   `json:"album"`
	URL        string  `json:"url"`
	Images     []Image `json:"image"`
	NowPlaying string  `json:"@attr"`
	Date       struct {
		UTS  string `json:"uts"`
		Text string `json:"#text"`
	} `json:"date"`
}

// AlbumInfo represents detailed album information
type AlbumInfo struct {
	Name          string `json:"name"`
	Artist        string `json:"artist"`
	URL           string `json:"url"`
	UserPlayCount string `json:"userplaycount"`
}

// ArtistInfo represents detailed artist information
type ArtistInfo struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Stats struct {
		UserPlays string `json:"userplaycount"`
	} `json:"stats"`
}

// TrackInfo represents detailed track information
type TrackInfo struct {
	Name          string `json:"name"`
	Artist        Artist `json:"artist"`
	Album         Album  `json:"album"`
	URL           string `json:"url"`
	UserPlayCount string `json:"userplaycount"`
}

// TopAlbumsResponse represents the response from user.getTopAlbums
type TopAlbumsResponse struct {
	TopAlbums struct {
		Albums []Album `json:"album"`
	} `json:"topalbums"`
}

// TopArtistsResponse represents the response from user.getTopArtists
type TopArtistsResponse struct {
	TopArtists struct {
		Artists []Artist `json:"artist"`
	} `json:"topartists"`
}

// TopTracksResponse represents the response from user.getTopTracks
type TopTracksResponse struct {
	TopTracks struct {
		Tracks []Track `json:"track"`
	} `json:"toptracks"`
}

// RecentTracksResponse represents the response from user.getRecentTracks
type RecentTracksResponse struct {
	RecentTracks struct {
		Tracks []Track `json:"track"`
	} `json:"recenttracks"`
}

// WeeklyArtistChartResponse represents the response from user.getWeeklyArtistChart
type WeeklyArtistChartResponse struct {
	WeeklyArtistChart struct {
		Artists []struct {
			Name      string `json:"name"`
			PlayCount string `json:"playcount"`
		} `json:"artist"`
	} `json:"weeklyartistchart"`
}

// AlbumInfoResponse represents the response from album.getInfo
type AlbumInfoResponse struct {
	Album AlbumInfo `json:"album"`
}

// ArtistInfoResponse represents the response from artist.getInfo
type ArtistInfoResponse struct {
	Artist ArtistInfo `json:"artist"`
}

// TrackInfoResponse represents the response from track.getInfo
type TrackInfoResponse struct {
	Track TrackInfo `json:"track"`
}

// User API methods

// GetTopAlbums gets top albums for a user
func (api *API) GetTopAlbums(ctx context.Context, params map[string]interface{}) (*TopAlbumsResponse, error) {
	body, err := api.makeRequest(ctx, "user.getTopAlbums", params)
	if err != nil {
		return nil, err
	}

	var response TopAlbumsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetTopArtists gets top artists for a user
func (api *API) GetTopArtists(ctx context.Context, params map[string]interface{}) (*TopArtistsResponse, error) {
	body, err := api.makeRequest(ctx, "user.getTopArtists", params)
	if err != nil {
		return nil, err
	}

	var response TopArtistsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetTopTracks gets top tracks for a user
func (api *API) GetTopTracks(ctx context.Context, params map[string]interface{}) (*TopTracksResponse, error) {
	body, err := api.makeRequest(ctx, "user.getTopTracks", params)
	if err != nil {
		return nil, err
	}

	var response TopTracksResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetRecentTracks gets recent tracks for a user
func (api *API) GetRecentTracks(ctx context.Context, params map[string]interface{}) (*RecentTracksResponse, error) {
	body, err := api.makeRequest(ctx, "user.getRecentTracks", params)
	if err != nil {
		return nil, err
	}

	var response RecentTracksResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetWeeklyArtistChart gets weekly artist chart for a user
func (api *API) GetWeeklyArtistChart(ctx context.Context, params map[string]interface{}) (*WeeklyArtistChartResponse, error) {
	body, err := api.makeRequest(ctx, "user.getWeeklyArtistChart", params)
	if err != nil {
		return nil, err
	}

	var response WeeklyArtistChartResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// Artist API methods

// GetArtistInfo gets information about an artist
func (api *API) GetArtistInfo(ctx context.Context, params map[string]interface{}) (*ArtistInfoResponse, error) {
	body, err := api.makeRequest(ctx, "artist.getInfo", params)
	if err != nil {
		return nil, err
	}

	var response ArtistInfoResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// Album API methods

// GetAlbumInfo gets information about an album
func (api *API) GetAlbumInfo(ctx context.Context, params map[string]interface{}) (*AlbumInfoResponse, error) {
	body, err := api.makeRequest(ctx, "album.getInfo", params)
	if err != nil {
		return nil, err
	}

	var response AlbumInfoResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// Track API methods

// GetTrackInfo gets information about a track
func (api *API) GetTrackInfo(ctx context.Context, params map[string]interface{}) (*TrackInfoResponse, error) {
	body, err := api.makeRequest(ctx, "track.getInfo", params)
	if err != nil {
		return nil, err
	}

	var response TrackInfoResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response, nil
}
