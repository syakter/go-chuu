package lastfm

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/syakter/go-chuu/internal/cache"
	"github.com/syakter/go-chuu/internal/config"
	"github.com/syakter/go-chuu/internal/errors"
	"github.com/syakter/go-chuu/internal/types"
)

// albumAggregation holds aggregated album statistics
type albumAggregation struct {
	TotalPlaycount int
	UserCount      int
}

// trackAggregation holds aggregated track statistics
type trackAggregation struct {
	TotalPlaycount int
	UserCount      int
}

// APIInterface defines the interface for Last.fm API operations
type APIInterface interface {
	GetTopAlbums(ctx context.Context, params map[string]interface{}) (*TopAlbumsResponse, error)
	GetTopArtists(ctx context.Context, params map[string]interface{}) (*TopArtistsResponse, error)
	GetTopTracks(ctx context.Context, params map[string]interface{}) (*TopTracksResponse, error)
	GetRecentTracks(ctx context.Context, params map[string]interface{}) (*RecentTracksResponse, error)
	GetWeeklyArtistChart(ctx context.Context, params map[string]interface{}) (*WeeklyArtistChartResponse, error)
	GetArtistInfo(ctx context.Context, params map[string]interface{}) (*ArtistInfoResponse, error)
	GetAlbumInfo(ctx context.Context, params map[string]interface{}) (*AlbumInfoResponse, error)
	GetTrackInfo(ctx context.Context, params map[string]interface{}) (*TrackInfoResponse, error)
}

// Client wraps the Last.fm API with enhanced functionality
type Client struct {
	api                   APIInterface
	cache                 cache.Cache
	config                *config.Config
	logger                *slog.Logger
	maxConcurrentRequests int
	semaphore             chan struct{}
}

// NewClient creates a new Last.fm client
func NewClient(cfg *config.Config, cache cache.Cache, logger *slog.Logger) *Client {
	api := NewAPI(cfg.LastFMAPIKey, cfg.LastFMAPISecret)

	c := &Client{
		api:                   api,
		cache:                 cache,
		config:                cfg,
		logger:                logger,
		maxConcurrentRequests: cfg.MaxConcurrentRequests,
		semaphore:             make(chan struct{}, cfg.MaxConcurrentRequests),
	}

	return c
}

// GetAPI returns the underlying Last.fm API client
func (c *Client) GetAPI() APIInterface {
	return c.api
}

// GetArtistScrobbles gets scrobble counts for an artist across all users
func (c *Client) GetArtistScrobbles(ctx context.Context, artistName string) ([]types.UserCount, error) {
	c.logger.Debug("Getting artist scrobbles", "artist", artistName)

	cacheKey := types.CacheKey{
		Type:   "artist_scrobbles",
		Artist: artistName,
	}

	result, err := cache.GetOrSet(c.cache, cacheKey, c.config.CacheTTL, func() (types.UserCounts, error) {
		userCounts, err := c.fetchArtistScrobbles(ctx, artistName)
		return types.UserCounts(userCounts), err
	})
	if err != nil {
		return nil, errors.NewAPIError(fmt.Sprintf("failed to get artist scrobbles for %s", artistName), err)
	}

	return []types.UserCount(result), nil
}

// fetchArtistScrobbles fetches artist scrobbles from the API
func (c *Client) fetchArtistScrobbles(ctx context.Context, artistName string) ([]types.UserCount, error) {
	artistName = strings.Replace(artistName, "&amp;", "\u0026", 1)

	counts := make(map[string]int)
	var wg sync.WaitGroup
	var mu sync.Mutex
	errorChan := make(chan error, len(c.config.Users))

	for _, user := range c.config.Users {
		wg.Add(1)
		go func(user string) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case c.semaphore <- struct{}{}:
				defer func() { <-c.semaphore }()
			case <-ctx.Done():
				errorChan <- ctx.Err()
				return
			}

			result, err := c.api.GetArtistInfo(ctx, map[string]interface{}{"artist": artistName, "username": user})
			if err != nil {
				c.logger.Warn("Failed to get artist info", "user", user, "artist", artistName, "error", err)
				// Send the error to the channel for potential propagation
				errorChan <- err
				mu.Lock()
				counts[user] = 0
				mu.Unlock()
				return
			}

			playcount := 0
			if result.Artist.Stats.UserPlays != "" {
				if pc, err := strconv.Atoi(result.Artist.Stats.UserPlays); err == nil {
					playcount = pc
				}
			}

			mu.Lock()
			counts[user] = playcount
			mu.Unlock()
		}(user)
	}

	wg.Wait()
	close(errorChan)

	// Check for context cancellation
	if ctx.Err() != nil {
		return nil, errors.NewTimeoutError("request cancelled", ctx.Err())
	}

	// Check if we should propagate API errors
	var apiErrors []error
	for err := range errorChan {
		if ctx.Err() == nil { // Don't collect context errors
			apiErrors = append(apiErrors, err)
		}
	}

	// If all users failed with API errors, log but don't propagate (resilient behavior)
	if len(apiErrors) > 0 && len(apiErrors) >= len(c.config.Users) {
		c.logger.Warn("All users failed for artist scrobbles", "artist", artistName, "errors", len(apiErrors))
		// Return empty results instead of propagating error
	}

	// Convert to sorted slice
	var userCounts []types.UserCount
	for user, count := range counts {
		userCounts = append(userCounts, types.UserCount{
			Username:  user,
			Playcount: count,
		})
	}

	sort.Slice(userCounts, func(i, j int) bool {
		return userCounts[i].Playcount > userCounts[j].Playcount
	})

	return userCounts, nil
}

// GetAlbumScrobbles gets scrobble counts for an album across all users
func (c *Client) GetAlbumScrobbles(ctx context.Context, artistName, albumName string) ([]types.UserCount, error) {
	c.logger.Debug("Getting album scrobbles", "artist", artistName, "album", albumName)

	cacheKey := types.CacheKey{
		Type:   "album_scrobbles",
		Artist: artistName,
		Album:  albumName,
	}

	result, err := cache.GetOrSet(c.cache, cacheKey, c.config.CacheTTL, func() (types.UserCounts, error) {
		userCounts, err := c.fetchAlbumScrobbles(ctx, artistName, albumName)
		return types.UserCounts(userCounts), err
	})
	if err != nil {
		return nil, errors.NewAPIError(fmt.Sprintf("failed to get album scrobbles for %s - %s", artistName, albumName), err)
	}

	return []types.UserCount(result), nil
}

// fetchAlbumScrobbles fetches album scrobbles from the API
func (c *Client) fetchAlbumScrobbles(ctx context.Context, artistName, albumName string) ([]types.UserCount, error) {
	artistName = strings.Replace(artistName, "&amp;", "\u0026", 1)
	albumName = strings.Replace(albumName, "&amp;", "\u0026", 1)

	counts := make(map[string]int)
	var wg sync.WaitGroup
	var mu sync.Mutex
	errorChan := make(chan error, len(c.config.Users))

	for _, user := range c.config.Users {
		wg.Add(1)
		go func(user string) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case c.semaphore <- struct{}{}:
				defer func() { <-c.semaphore }()
			case <-ctx.Done():
				errorChan <- ctx.Err()
				return
			}

			result, err := c.api.GetAlbumInfo(ctx, map[string]interface{}{"artist": artistName, "album": albumName, "username": user})
			if err != nil {
				c.logger.Warn("Failed to get album info", "user", user, "artist", artistName, "album", albumName, "error", err)
				// Send the error to the channel for potential propagation
				errorChan <- err
				mu.Lock()
				counts[user] = 0
				mu.Unlock()
				return
			}

			playcount := 0
			if result.Album.UserPlayCount != "" {
				if pc, err := strconv.Atoi(result.Album.UserPlayCount); err == nil {
					playcount = pc
				}
			}

			mu.Lock()
			counts[user] = playcount
			mu.Unlock()
		}(user)
	}

	wg.Wait()
	close(errorChan)

	if ctx.Err() != nil {
		return nil, errors.NewTimeoutError("request cancelled", ctx.Err())
	}

	// Check if we should propagate API errors
	var apiErrors []error
	for err := range errorChan {
		if ctx.Err() == nil { // Don't collect context errors
			apiErrors = append(apiErrors, err)
		}
	}

	// If all users failed with API errors, log but don't propagate (resilient behavior)
	if len(apiErrors) > 0 && len(apiErrors) >= len(c.config.Users) {
		c.logger.Warn("All users failed for album scrobbles", "artist", artistName, "album", albumName, "errors", len(apiErrors))
		// Return empty results instead of propagating error
	}

	var userCounts []types.UserCount
	for user, count := range counts {
		userCounts = append(userCounts, types.UserCount{
			Username:  user,
			Playcount: count,
		})
	}

	sort.Slice(userCounts, func(i, j int) bool {
		return userCounts[i].Playcount > userCounts[j].Playcount
	})

	return userCounts, nil
}

// GetTrackScrobbles gets scrobble counts for a track across all users
func (c *Client) GetTrackScrobbles(ctx context.Context, artistName, trackName string) ([]types.UserCount, error) {
	c.logger.Debug("Getting track scrobbles", "artist", artistName, "track", trackName)

	cacheKey := types.CacheKey{
		Type:   "track_scrobbles",
		Artist: artistName,
		Track:  trackName,
	}

	result, err := cache.GetOrSet(c.cache, cacheKey, c.config.CacheTTL, func() (types.UserCounts, error) {
		userCounts, err := c.fetchTrackScrobbles(ctx, artistName, trackName)
		return types.UserCounts(userCounts), err
	})
	if err != nil {
		return nil, errors.NewAPIError(fmt.Sprintf("failed to get track scrobbles for %s - %s", artistName, trackName), err)
	}

	return []types.UserCount(result), nil
}

// fetchTrackScrobbles fetches track scrobbles from the API
func (c *Client) fetchTrackScrobbles(ctx context.Context, artistName, trackName string) ([]types.UserCount, error) {
	artistName = strings.Replace(artistName, "&amp;", "\u0026", 1)
	trackName = strings.Replace(trackName, "&amp;", "\u0026", 1)

	counts := make(map[string]int)
	var wg sync.WaitGroup
	var mu sync.Mutex
	errorChan := make(chan error, len(c.config.Users))

	for _, user := range c.config.Users {
		wg.Add(1)
		go func(user string) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case c.semaphore <- struct{}{}:
				defer func() { <-c.semaphore }()
			case <-ctx.Done():
				errorChan <- ctx.Err()
				return
			}

			result, err := c.api.GetTrackInfo(ctx, map[string]interface{}{"artist": artistName, "track": trackName, "username": user})
			if err != nil {
				c.logger.Warn("Failed to get track info", "user", user, "artist", artistName, "track", trackName, "error", err)
				// Send the error to the channel for potential propagation
				errorChan <- err
				mu.Lock()
				counts[user] = 0
				mu.Unlock()
				return
			}

			playcount := 0
			if result.Track.UserPlayCount != "" {
				if pc, err := strconv.Atoi(result.Track.UserPlayCount); err == nil {
					playcount = pc
				}
			}

			mu.Lock()
			counts[user] = playcount
			mu.Unlock()
		}(user)
	}

	wg.Wait()
	close(errorChan)

	if ctx.Err() != nil {
		return nil, errors.NewTimeoutError("request cancelled", ctx.Err())
	}

	// Check if we should propagate API errors
	var apiErrors []error
	for err := range errorChan {
		if ctx.Err() == nil { // Don't collect context errors
			apiErrors = append(apiErrors, err)
		}
	}

	// If all users failed with API errors, log but don't propagate (resilient behavior)
	if len(apiErrors) > 0 && len(apiErrors) >= len(c.config.Users) {
		c.logger.Warn("All users failed for track scrobbles", "artist", artistName, "track", trackName, "errors", len(apiErrors))
		// Return empty results instead of propagating error
	}

	var userCounts []types.UserCount
	for user, count := range counts {
		userCounts = append(userCounts, types.UserCount{
			Username:  user,
			Playcount: count,
		})
	}

	sort.Slice(userCounts, func(i, j int) bool {
		return userCounts[i].Playcount > userCounts[j].Playcount
	})

	return userCounts, nil
}

// GetNowPlaying gets currently playing tracks for all users
func (c *Client) GetNowPlaying(ctx context.Context) (map[string]string, error) {
	c.logger.Debug("Getting now playing for all users")

	nowPlaying := make(map[string]string)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, user := range c.config.Users {
		wg.Add(1)
		go func(user string) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case c.semaphore <- struct{}{}:
				defer func() { <-c.semaphore }()
			case <-ctx.Done():
				return
			}

			result, err := c.api.GetRecentTracks(ctx, map[string]interface{}{"user": user, "limit": 1})
			if err != nil {
				c.logger.Warn("Failed to get recent tracks", "user", user, "error", err)
				return
			}

			if len(result.RecentTracks.Tracks) > 0 {
				track := result.RecentTracks.Tracks[0]
				if track.IsNowPlaying() {
					trackInfo := fmt.Sprintf("%s - %s", track.Artist.Name, track.Name)
					mu.Lock()
					nowPlaying[user] = trackInfo
					mu.Unlock()
				}
			}
		}(user)
	}

	wg.Wait()

	if ctx.Err() != nil {
		return nil, errors.NewTimeoutError("request cancelled", ctx.Err())
	}

	return nowPlaying, nil
}

// GetUserTopTracks gets top tracks for a user
func (c *Client) GetUserTopTracks(ctx context.Context, username, period string, limit int) ([]string, error) {
	c.logger.Debug("Getting user top tracks", "user", username, "period", period, "limit", limit)

	cacheKey := types.CacheKey{
		Type:   "user_top_tracks",
		User:   username,
		Period: period,
		Limit:  limit,
	}

	result, err := cache.GetOrSet(c.cache, cacheKey, c.config.CacheTTL, func() (types.StringSlice, error) {
		tracks, err := c.fetchUserTopTracks(ctx, username, period, limit)
		return types.StringSlice(tracks), err
	})
	if err != nil {
		return nil, errors.NewAPIError(fmt.Sprintf("failed to get top tracks for %s", username), err)
	}

	return []string(result), nil
}

// GetUserTopAlbums gets top albums for a user
func (c *Client) GetUserTopAlbums(ctx context.Context, username, period string, limit int) ([]string, error) {
	c.logger.Debug("Getting user top albums", "user", username, "period", period, "limit", limit)

	cacheKey := types.CacheKey{
		Type:   "user_top_albums",
		User:   username,
		Period: period,
		Limit:  limit,
	}

	result, err := cache.GetOrSet(c.cache, cacheKey, c.config.CacheTTL, func() (types.StringSlice, error) {
		albums, err := c.fetchUserTopAlbums(ctx, username, period, limit)
		return types.StringSlice(albums), err
	})
	if err != nil {
		return nil, errors.NewAPIError(fmt.Sprintf("failed to get top albums for %s", username), err)
	}

	return []string(result), nil
}

// fetchUserTopAlbums fetches user top albums from the API
func (c *Client) fetchUserTopAlbums(ctx context.Context, username, period string, limit int) ([]string, error) {
	period = c.normalizePeriod(period)

	result, err := c.api.GetTopAlbums(ctx, map[string]interface{}{"user": username, "period": period, "limit": limit})
	if err != nil {
		return nil, err
	}

	var albums []string
	for i, album := range result.TopAlbums.Albums {
		if i >= limit {
			break
		}
		albums = append(albums, fmt.Sprintf("%s - %s", album.Artist.Name, album.Name))
	}

	return albums, nil
}

// fetchUserTopAlbumsWithPlaycounts fetches user top albums with playcount data
func (c *Client) fetchUserTopAlbumsWithPlaycounts(ctx context.Context, username, period string, limit int) ([]Album, error) {
	period = c.normalizePeriod(period)

	result, err := c.api.GetTopAlbums(ctx, map[string]interface{}{"user": username, "period": period, "limit": limit})
	if err != nil {
		return nil, err
	}

	var albums []Album
	for i, album := range result.TopAlbums.Albums {
		if i >= limit {
			break
		}
		albums = append(albums, album)
	}

	return albums, nil
}

// fetchUserTopTracksWithPlaycounts fetches user top tracks with playcount data
func (c *Client) fetchUserTopTracksWithPlaycounts(ctx context.Context, username, period string, limit int) ([]Track, error) {
	period = c.normalizePeriod(period)

	result, err := c.api.GetTopTracks(ctx, map[string]interface{}{"user": username, "period": period, "limit": limit})
	if err != nil {
		return nil, err
	}

	var tracks []Track
	for i, track := range result.TopTracks.Tracks {
		if i >= limit {
			break
		}
		tracks = append(tracks, track)
	}

	return tracks, nil
}

// GetUserTopArtists gets top artists for a user
func (c *Client) GetUserTopArtists(ctx context.Context, username, period string, limit int) ([]string, error) {
	c.logger.Debug("Getting user top artists", "user", username, "period", period, "limit", limit)

	cacheKey := types.CacheKey{
		Type:   "user_top_artists",
		User:   username,
		Period: period,
		Limit:  limit,
	}

	result, err := cache.GetOrSet(c.cache, cacheKey, c.config.CacheTTL, func() (types.StringSlice, error) {
		artists, err := c.fetchUserTopArtists(ctx, username, period, limit)
		return types.StringSlice(artists), err
	})
	if err != nil {
		return nil, errors.NewAPIError(fmt.Sprintf("failed to get top artists for %s", username), err)
	}

	return []string(result), nil
}

// fetchUserTopArtists fetches user top artists from the API
func (c *Client) fetchUserTopArtists(ctx context.Context, username, period string, limit int) ([]string, error) {
	period = c.normalizePeriod(period)

	result, err := c.api.GetTopArtists(ctx, map[string]interface{}{"user": username, "period": period, "limit": limit})
	if err != nil {
		return nil, err
	}

	var artists []string
	for i, artist := range result.TopArtists.Artists {
		if i >= limit {
			break
		}
		artists = append(artists, artist.Name)
	}

	return artists, nil
}

// GetUserRecentTracks gets recent tracks for a user
func (c *Client) GetUserRecentTracks(ctx context.Context, username string, limit int) ([]string, error) {
	c.logger.Debug("Getting user recent tracks", "user", username, "limit", limit)

	cacheKey := types.CacheKey{
		Type:  "user_recent_tracks",
		User:  username,
		Limit: limit,
	}

	result, err := cache.GetOrSet(c.cache, cacheKey, time.Minute*2, func() (types.StringSlice, error) {
		tracks, err := c.fetchUserRecentTracks(ctx, username, limit)
		return types.StringSlice(tracks), err
	})
	if err != nil {
		return nil, errors.NewAPIError(fmt.Sprintf("failed to get recent tracks for %s", username), err)
	}

	return []string(result), nil
}

// fetchUserRecentTracks fetches user recent tracks from the API
func (c *Client) fetchUserRecentTracks(ctx context.Context, username string, limit int) ([]string, error) {
	result, err := c.api.GetRecentTracks(ctx, map[string]interface{}{"user": username, "limit": limit})
	if err != nil {
		return nil, err
	}

	var tracks []string
	for i, track := range result.RecentTracks.Tracks {
		if i >= limit {
			break
		}
		// Skip now playing tracks in recent tracks list
		if track.IsNowPlaying() {
			continue
		}
		tracks = append(tracks, fmt.Sprintf("%s - %s", track.Artist.Name, track.Name))
	}

	return tracks, nil
}

// fetchUserTopTracks fetches user top tracks from the API
func (c *Client) fetchUserTopTracks(ctx context.Context, username, period string, limit int) ([]string, error) {
	period = c.normalizePeriod(period)

	result, err := c.api.GetTopTracks(ctx, map[string]interface{}{"user": username, "period": period, "limit": limit})
	if err != nil {
		return nil, err
	}

	var tracks []string
	for i, track := range result.TopTracks.Tracks {
		if i >= limit {
			break
		}
		tracks = append(tracks, fmt.Sprintf("%s - %s", track.Artist.Name, track.Name))
	}

	return tracks, nil
}

// GetWeeklyLeaderboard gets weekly scrobble leaderboard
func (c *Client) GetWeeklyLeaderboard(ctx context.Context) ([]types.LeaderboardEntry, error) {
	c.logger.Debug("Getting weekly leaderboard")

	cacheKey := types.CacheKey{
		Type:   "weekly_leaderboard",
		Period: "7day",
	}

	result, err := cache.GetOrSet(c.cache, cacheKey, time.Hour, func() (types.LeaderboardEntries, error) {
		entries, err := c.fetchWeeklyLeaderboard(ctx)
		return types.LeaderboardEntries(entries), err
	})
	if err != nil {
		return nil, errors.NewAPIError("failed to get weekly leaderboard", err)
	}

	return []types.LeaderboardEntry(result), nil
}

// fetchWeeklyLeaderboard fetches weekly leaderboard from the API
func (c *Client) fetchWeeklyLeaderboard(ctx context.Context) ([]types.LeaderboardEntry, error) {
	toTime := time.Now()
	fromTime := toTime.AddDate(0, 0, -7)

	counts := make(map[string]int)
	var wg sync.WaitGroup
	var mu sync.Mutex
	errorChan := make(chan error, len(c.config.Users))

	for _, user := range c.config.Users {
		wg.Add(1)
		go func(user string) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case c.semaphore <- struct{}{}:
				defer func() { <-c.semaphore }()
			case <-ctx.Done():
				errorChan <- ctx.Err()
				return
			}

			fromTimestamp := strconv.FormatInt(fromTime.Unix(), 10)
			toTimestamp := strconv.FormatInt(toTime.Unix(), 10)

			artistChart, err := c.api.GetWeeklyArtistChart(ctx, map[string]interface{}{"user": user, "from": fromTimestamp, "to": toTimestamp})
			if err != nil {
				c.logger.Warn("Failed to get weekly artist chart", "user", user, "error", err)
				// Send the error to the channel for potential propagation
				errorChan <- err
				return
			}

			totalPlayCount := 0
			for _, artist := range artistChart.WeeklyArtistChart.Artists {
				if playcount, err := strconv.Atoi(artist.PlayCount); err == nil {
					totalPlayCount += playcount
				}
			}

			mu.Lock()
			counts[user] = totalPlayCount
			mu.Unlock()
		}(user)
	}

	wg.Wait()
	close(errorChan)

	if ctx.Err() != nil {
		return nil, errors.NewTimeoutError("request cancelled", ctx.Err())
	}

	// Check if we should propagate API errors
	var apiErrors []error
	for err := range errorChan {
		if ctx.Err() == nil { // Don't collect context errors
			apiErrors = append(apiErrors, err)
		}
	}

	// If all users failed with API errors, log but don't propagate (resilient behavior)
	if len(apiErrors) > 0 && len(apiErrors) >= len(c.config.Users) {
		c.logger.Warn("All users failed for weekly leaderboard", "errors", len(apiErrors))
		// Return empty results instead of propagating error
	}

	var entries []types.LeaderboardEntry
	for user, count := range counts {
		entries = append(entries, types.LeaderboardEntry{
			Username:   user,
			Scrobbles:  count,
			PeriodFrom: fromTime,
			PeriodTo:   toTime,
		})
	}

	// Sort by scrobbles (descending) and assign ranks
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Scrobbles > entries[j].Scrobbles
	})

	for i := range entries {
		entries[i].Rank = i + 1
	}

	return entries, nil
}

// GetTopAlbumsAcrossUsers aggregates top albums across all users for a given period
func (c *Client) GetTopAlbumsAcrossUsers(ctx context.Context, period string, limit int) ([]types.AlbumCount, error) {
	c.logger.Debug("Getting top albums across all users", "period", period, "limit", limit)

	cacheKey := types.CacheKey{
		Type:   "top_albums_all",
		Period: period,
		Limit:  limit,
	}

	result, err := cache.GetOrSet(c.cache, cacheKey, c.config.CacheTTL, func() (types.AlbumCounts, error) {
		albums, err := c.fetchTopAlbumsAcrossUsers(ctx, period, limit)
		return types.AlbumCounts(albums), err
	})
	if err != nil {
		return nil, errors.NewAPIError("failed to get top albums across users", err)
	}

	return []types.AlbumCount(result), nil
}

// fetchTopAlbumsAcrossUsers fetches and aggregates top albums from all users
func (c *Client) fetchTopAlbumsAcrossUsers(ctx context.Context, period string, limit int) ([]types.AlbumCount, error) {
	albumStats := make(map[string]*albumAggregation)
	var wg sync.WaitGroup
	var mu sync.Mutex
	errorChan := make(chan error, len(c.config.Users))

	for _, user := range c.config.Users {
		wg.Add(1)
		go func(user string) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case c.semaphore <- struct{}{}:
				defer func() { <-c.semaphore }()
			case <-ctx.Done():
				return
			}

			albums, err := c.fetchUserTopAlbumsWithPlaycounts(ctx, user, period, 50)
			if err != nil {
				c.logger.Warn("Failed to get top albums for user", "user", user, "error", err)
				errorChan <- err
				return
			}

			mu.Lock()
			for _, album := range albums {
				albumName := fmt.Sprintf("%s - %s", album.Artist.Name, album.Name)
				playcount := 0
				if pc, err := strconv.Atoi(album.PlayCount); err == nil {
					playcount = pc
				}

				if existing, ok := albumStats[albumName]; ok {
					existing.TotalPlaycount += playcount
					existing.UserCount++
				} else {
					albumStats[albumName] = &albumAggregation{
						TotalPlaycount: playcount,
						UserCount:      1,
					}
				}
			}
			mu.Unlock()
		}(user)
	}

	wg.Wait()
	close(errorChan)

	// Check for errors
	for err := range errorChan {
		if err != nil {
			return nil, err
		}
	}

	// Convert map to sorted slice
	var result []types.AlbumCount
	for albumName, stats := range albumStats {
		result = append(result, types.AlbumCount{
			AlbumName: albumName,
			Playcount: stats.TotalPlaycount,
			UserCount: stats.UserCount,
		})
	}

	// Sort by total playcount descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].Playcount > result[j].Playcount
	})

	// Limit results
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result, nil
}

// GetTopTracksAcrossUsers aggregates top tracks across all users for a given period
func (c *Client) GetTopTracksAcrossUsers(ctx context.Context, period string, limit int) ([]types.TrackCount, error) {
	c.logger.Debug("Getting top tracks across all users", "period", period, "limit", limit)

	cacheKey := types.CacheKey{
		Type:   "top_tracks_all",
		Period: period,
		Limit:  limit,
	}

	result, err := cache.GetOrSet(c.cache, cacheKey, c.config.CacheTTL, func() (types.TrackCounts, error) {
		tracks, err := c.fetchTopTracksAcrossUsers(ctx, period, limit)
		return types.TrackCounts(tracks), err
	})
	if err != nil {
		return nil, errors.NewAPIError("failed to get top tracks across users", err)
	}

	return []types.TrackCount(result), nil
}

// fetchTopTracksAcrossUsers fetches and aggregates top tracks from all users
func (c *Client) fetchTopTracksAcrossUsers(ctx context.Context, period string, limit int) ([]types.TrackCount, error) {
	trackStats := make(map[string]*trackAggregation)
	var wg sync.WaitGroup
	var mu sync.Mutex
	errorChan := make(chan error, len(c.config.Users))

	for _, user := range c.config.Users {
		wg.Add(1)
		go func(user string) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case c.semaphore <- struct{}{}:
				defer func() { <-c.semaphore }()
			case <-ctx.Done():
				return
			}

			tracks, err := c.fetchUserTopTracksWithPlaycounts(ctx, user, period, 50)
			if err != nil {
				c.logger.Warn("Failed to get top tracks for user", "user", user, "error", err)
				errorChan <- err
				return
			}

			mu.Lock()
			for _, track := range tracks {
				trackName := fmt.Sprintf("%s - %s", track.Artist.Name, track.Name)
				playcount := 0
				if pc, err := strconv.Atoi(track.PlayCount); err == nil {
					playcount = pc
				}

				if existing, ok := trackStats[trackName]; ok {
					existing.TotalPlaycount += playcount
					existing.UserCount++
				} else {
					trackStats[trackName] = &trackAggregation{
						TotalPlaycount: playcount,
						UserCount:      1,
					}
				}
			}
			mu.Unlock()
		}(user)
	}

	wg.Wait()
	close(errorChan)

	// Check for errors
	for err := range errorChan {
		if err != nil {
			return nil, err
		}
	}

	// Convert map to sorted slice
	var result []types.TrackCount
	for trackName, stats := range trackStats {
		result = append(result, types.TrackCount{
			TrackName: trackName,
			Playcount: stats.TotalPlaycount,
			UserCount: stats.UserCount,
		})
	}

	// Sort by total playcount descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].Playcount > result[j].Playcount
	})

	// Limit results
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result, nil
}

// GetUserTopAlbumsByArtist gets top albums by a specific artist for a user
func (c *Client) GetUserTopAlbumsByArtist(ctx context.Context, username, artistName string, limit int) ([]string, error) {
	c.logger.Debug("Getting user top albums by artist", "user", username, "artist", artistName, "limit", limit)

	cacheKey := types.CacheKey{
		Type:   "user_albums_by_artist",
		User:   username,
		Artist: artistName,
		Limit:  limit,
	}

	result, err := cache.GetOrSet(c.cache, cacheKey, c.config.CacheTTL, func() (types.StringSlice, error) {
		albums, err := c.fetchUserTopAlbumsByArtist(ctx, username, artistName, limit)
		return types.StringSlice(albums), err
	})
	if err != nil {
		return nil, errors.NewAPIError(fmt.Sprintf("failed to get albums by %s for user %s", artistName, username), err)
	}

	return []string(result), nil
}

// fetchUserTopAlbumsByArtist fetches top albums by artist for a specific user
func (c *Client) fetchUserTopAlbumsByArtist(ctx context.Context, username, artistName string, limit int) ([]string, error) {
	// Get all top albums for the user and filter by artist
	allAlbums, err := c.GetUserTopAlbums(ctx, username, "overall", 200) // Get more albums to filter
	if err != nil {
		return nil, err
	}

	var filteredAlbums []string
	artistLower := strings.ToLower(artistName)

	for _, album := range allAlbums {
		// Check if album contains the artist name (albums are typically formatted as "Album Name by Artist Name")
		if strings.Contains(strings.ToLower(album), artistLower) {
			filteredAlbums = append(filteredAlbums, album)
			if limit > 0 && len(filteredAlbums) >= limit {
				break
			}
		}
	}

	return filteredAlbums, nil
}

// GetUserTopTracksByArtist gets top tracks by a specific artist for a user
func (c *Client) GetUserTopTracksByArtist(ctx context.Context, username, artistName string, limit int) ([]string, error) {
	c.logger.Debug("Getting user top tracks by artist", "user", username, "artist", artistName, "limit", limit)

	cacheKey := types.CacheKey{
		Type:   "user_tracks_by_artist",
		User:   username,
		Artist: artistName,
		Limit:  limit,
	}

	result, err := cache.GetOrSet(c.cache, cacheKey, c.config.CacheTTL, func() (types.StringSlice, error) {
		tracks, err := c.fetchUserTopTracksByArtist(ctx, username, artistName, limit)
		return types.StringSlice(tracks), err
	})
	if err != nil {
		return nil, errors.NewAPIError(fmt.Sprintf("failed to get tracks by %s for user %s", artistName, username), err)
	}

	return []string(result), nil
}

// fetchUserTopTracksByArtist fetches top tracks by artist for a specific user
func (c *Client) fetchUserTopTracksByArtist(ctx context.Context, username, artistName string, limit int) ([]string, error) {
	// Get all top tracks for the user and filter by artist
	allTracks, err := c.GetUserTopTracks(ctx, username, "overall", 200) // Get more tracks to filter
	if err != nil {
		return nil, err
	}

	var filteredTracks []string
	artistLower := strings.ToLower(artistName)

	for _, track := range allTracks {
		// Check if track contains the artist name (tracks are typically formatted as "Track Name by Artist Name")
		if strings.Contains(strings.ToLower(track), artistLower) {
			filteredTracks = append(filteredTracks, track)
			if limit > 0 && len(filteredTracks) >= limit {
				break
			}
		}
	}

	return filteredTracks, nil
}

// normalizePeriod converts user-friendly periods to Last.fm API periods
func (c *Client) normalizePeriod(period string) string {
	switch strings.ToLower(period) {
	case "7d", "1w":
		return "7day"
	case "1m", "30d":
		return "1month"
	case "3m", "90d":
		return "3month"
	case "6m", "180d":
		return "6month"
	case "1y", "365d":
		return "12month"
	default:
		return "overall"
	}
}
