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

// artistAggregation holds aggregated artist statistics with per-user playcounts
type artistAggregation struct {
	TotalPlaycount int
	UserCount      int
	UserPlaycounts map[string]int // username -> playcount
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
	GetArtistTopAlbums(ctx context.Context, params map[string]interface{}) (*TopAlbumsResponse, error)
	GetArtistTopTracks(ctx context.Context, params map[string]interface{}) (*TopTracksResponse, error)
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
				if track.NowPlaying == "true" {
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

	// Use shorter TTL for 24h periods
	cacheTTL := c.config.CacheTTL
	if is24HourPeriod(period) {
		cacheTTL = time.Minute * 30 // 30 minutes for 24h periods
	}

	result, err := cache.GetOrSet(c.cache, cacheKey, cacheTTL, func() (types.StringSlice, error) {
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

	// Use shorter TTL for 24h periods
	cacheTTL := c.config.CacheTTL
	if is24HourPeriod(period) {
		cacheTTL = time.Minute * 30 // 30 minutes for 24h periods
	}

	result, err := cache.GetOrSet(c.cache, cacheKey, cacheTTL, func() (types.StringSlice, error) {
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
	originalPeriod := period
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
		albums = append(albums, formatAlbumName(album.Artist.Name, album.Name))
	}

	// Apply 24h filtering if needed
	return c.filterRecent24Hours(albums, originalPeriod), nil
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

	// Use shorter TTL for 24h periods
	cacheTTL := c.config.CacheTTL
	if is24HourPeriod(period) {
		cacheTTL = time.Minute * 30 // 30 minutes for 24h periods
	}

	result, err := cache.GetOrSet(c.cache, cacheKey, cacheTTL, func() (types.StringSlice, error) {
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
	originalPeriod := period
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

	// Apply 24h filtering if needed
	return c.filterRecent24Hours(artists, originalPeriod), nil
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
		if track.NowPlaying == "true" {
			continue
		}
		tracks = append(tracks, formatTrackName(track.Artist.Name, track.Name))
	}

	return tracks, nil
}

// fetchUserTopTracks fetches user top tracks from the API
func (c *Client) fetchUserTopTracks(ctx context.Context, username, period string, limit int) ([]string, error) {
	originalPeriod := period
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
		tracks = append(tracks, formatTrackName(track.Artist.Name, track.Name))
	}

	// Apply 24h filtering if needed
	return c.filterRecent24Hours(tracks, originalPeriod), nil
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
				albumName := formatAlbumName(album.Artist.Name, album.Name)
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
				trackName := formatTrackName(track.Artist.Name, track.Name)
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
	// Try artist-specific API first
	artistAlbums, err := c.fetchArtistAlbumsWithUserPlaycounts(ctx, username, artistName, limit)
	if err == nil && len(artistAlbums) > 0 {
		c.logger.Debug("Successfully fetched artist albums via artist API", "artist", artistName, "count", len(artistAlbums))
		return artistAlbums, nil
	}

	// Log the artist API failure and fall back to filtering method
	c.logger.Debug("Artist API failed, falling back to filtering", "artist", artistName, "error", err)

	// Fallback: Get all top albums for the user and filter by artist
	allAlbums, err := c.GetUserTopAlbums(ctx, username, "overall", 200) // Get more albums to filter
	if err != nil {
		return nil, err
	}

	var filteredAlbums []string
	artistLower := strings.ToLower(strings.TrimSpace(artistName))

	for _, albumStr := range allAlbums {
		// Parse the "Artist - Album" format (output format)
		parts := strings.SplitN(albumStr, " - ", 2)
		if len(parts) == 2 {
			albumArtist := strings.TrimSpace(parts[0])
			// Compare artist names directly
			if strings.EqualFold(albumArtist, artistLower) {
				filteredAlbums = append(filteredAlbums, albumStr)
			}
		} else {
			// If parsing fails, fall back to old string matching
			if strings.Contains(strings.ToLower(albumStr), artistLower) {
				filteredAlbums = append(filteredAlbums, albumStr)
			}
		}

		if limit > 0 && len(filteredAlbums) >= limit {
			break
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
	// Try artist-specific API first
	artistTracks, err := c.fetchArtistTracksWithUserPlaycounts(ctx, username, artistName, limit)
	if err == nil && len(artistTracks) > 0 {
		c.logger.Debug("Successfully fetched artist tracks via artist API", "artist", artistName, "count", len(artistTracks))
		return artistTracks, nil
	}

	// Log the artist API failure and fall back to filtering method
	c.logger.Debug("Artist API failed, falling back to filtering", "artist", artistName, "error", err)

	// Fallback: Get all top tracks for the user and filter by artist
	allTracks, err := c.GetUserTopTracks(ctx, username, "overall", 200) // Get more tracks to filter
	if err != nil {
		return nil, err
	}

	var filteredTracks []string
	artistLower := strings.ToLower(strings.TrimSpace(artistName))

	for _, trackStr := range allTracks {
		// Parse the "Artist - Track" format (output format)
		parts := strings.SplitN(trackStr, " - ", 2)
		if len(parts) == 2 {
			trackArtist := strings.TrimSpace(parts[0])
			// Compare artist names directly
			if strings.EqualFold(trackArtist, artistLower) {
				filteredTracks = append(filteredTracks, trackStr)
			}
		} else {
			// If parsing fails, fall back to old string matching
			if strings.Contains(strings.ToLower(trackStr), artistLower) {
				filteredTracks = append(filteredTracks, trackStr)
			}
		}

		if limit > 0 && len(filteredTracks) >= limit {
			break
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
	case "24h":
		return "7day" // Use 7day as base, will be filtered later
	default:
		return "overall"
	}
}

// formatAlbumName creates a consistent "Artist - Album" format for output display
func formatAlbumName(artist, album string) string {
	return fmt.Sprintf("%s - %s", artist, album)
}

// formatTrackName creates a consistent "Artist - Track" format for output display
func formatTrackName(artist, track string) string {
	return fmt.Sprintf("%s - %s", artist, track)
}

// parseAlbumArtist parses "Album by Artist" format back to components
func parseAlbumArtist(formatted string) (artist, album string, err error) {
	parts := strings.SplitN(formatted, " by ", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid album format: expected 'Album by Artist', got '%s'", formatted)
	}
	return strings.TrimSpace(parts[1]), strings.TrimSpace(parts[0]), nil
}

// parseTrackArtist parses "Track by Artist" format back to components
func parseTrackArtist(formatted string) (artist, track string, err error) {
	parts := strings.SplitN(formatted, " by ", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid track format: expected 'Track by Artist', got '%s'", formatted)
	}
	return strings.TrimSpace(parts[1]), strings.TrimSpace(parts[0]), nil
}

// is24HourPeriod checks if the given period is a 24-hour period
func is24HourPeriod(period string) bool {
	return strings.ToLower(period) == "24h"
}

// filterRecent24Hours filters tracks/data to only include items from the last 24 hours
// This is a helper function for 24h period support since Last.fm doesn't have native 24h API
func (c *Client) filterRecent24Hours(items []string, period string) []string {
	if !is24HourPeriod(period) {
		return items
	}

	// For 24h periods, we want to limit the results more aggressively
	// Since we can't get actual timestamps from top tracks/albums APIs,
	// we'll return fewer results to approximate "recent" listening
	maxItems := min(len(items), 5) // Show only top 5 for 24h
	return items[:maxItems]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// fetchArtistAlbumsWithUserPlaycounts gets artist albums and cross-references with user listening data
func (c *Client) fetchArtistAlbumsWithUserPlaycounts(ctx context.Context, username, artistName string, limit int) ([]string, error) {
	// Get top albums for this artist
	artistAlbumsResp, err := c.api.GetArtistTopAlbums(ctx, map[string]interface{}{
		"artist": artistName,
		"limit":  limit * 2, // Get more to ensure we have enough after filtering
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get artist albums: %w", err)
	}

	if len(artistAlbumsResp.TopAlbums.Albums) == 0 {
		return []string{}, nil
	}

	// Get user's top albums to cross-reference playcount data
	userAlbums, err := c.GetUserTopAlbums(ctx, username, "overall", 500)
	if err != nil {
		c.logger.Warn("Failed to get user albums for cross-reference", "error", err)
		// Continue without playcount data
	}

	// Create a map of user's albums for quick lookup
	userAlbumMap := make(map[string]bool)
	for _, album := range userAlbums {
		userAlbumMap[strings.ToLower(album)] = true
	}

	// Build result list with albums the user has listened to
	var result []string
	for _, album := range artistAlbumsResp.TopAlbums.Albums {
		albumFormatted := formatAlbumName(album.Artist.Name, album.Name)

		// Only include albums the user has actually listened to
		if userAlbumMap[strings.ToLower(albumFormatted)] {
			result = append(result, albumFormatted)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}

	return result, nil
}

// fetchUserTopArtistsWithPlaycounts fetches user top artists with playcount data
func (c *Client) fetchUserTopArtistsWithPlaycounts(ctx context.Context, username, period string, limit int) ([]Artist, error) {
	period = c.normalizePeriod(period)

	result, err := c.api.GetTopArtists(ctx, map[string]interface{}{"user": username, "period": period, "limit": limit})
	if err != nil {
		return nil, err
	}

	var artists []Artist
	for i, artist := range result.TopArtists.Artists {
		if i >= limit {
			break
		}
		artists = append(artists, artist)
	}

	return artists, nil
}

// GetGroupRecommendations returns artists the group loves that the given user hasn't listened to much
func (c *Client) GetGroupRecommendations(ctx context.Context, username, period string) ([]types.RecommendedArtist, error) {
	c.logger.Debug("Getting group recommendations", "user", username, "period", period)

	cacheKey := types.CacheKey{
		Type:   "group_recommendations",
		User:   username,
		Period: period,
	}

	result, err := cache.GetOrSet(c.cache, cacheKey, c.config.CacheTTL, func() (types.RecommendedArtists, error) {
		artists, err := c.fetchGroupRecommendations(ctx, username, period)
		return types.RecommendedArtists(artists), err
	})
	if err != nil {
		return nil, errors.NewAPIError(fmt.Sprintf("failed to get group recommendations for %s", username), err)
	}

	return []types.RecommendedArtist(result), nil
}

// fetchGroupRecommendations fetches and computes group recommendations
func (c *Client) fetchGroupRecommendations(ctx context.Context, username, period string) ([]types.RecommendedArtist, error) {
	artistStats := make(map[string]*artistAggregation)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, user := range c.config.Users {
		wg.Add(1)
		go func(user string) {
			defer wg.Done()

			select {
			case c.semaphore <- struct{}{}:
				defer func() { <-c.semaphore }()
			case <-ctx.Done():
				return
			}

			artists, err := c.fetchUserTopArtistsWithPlaycounts(ctx, user, period, 100)
			if err != nil {
				c.logger.Warn("Failed to get top artists for user", "user", user, "error", err)
				return
			}

			mu.Lock()
			for _, artist := range artists {
				playcount := 0
				if pc, err := strconv.Atoi(artist.PlayCount); err == nil {
					playcount = pc
				}
				key := strings.ToLower(artist.Name)
				if existing, ok := artistStats[key]; ok {
					existing.TotalPlaycount += playcount
					existing.UserCount++
					existing.UserPlaycounts[user] = playcount
				} else {
					artistStats[key] = &artistAggregation{
						TotalPlaycount: playcount,
						UserCount:      1,
						UserPlaycounts: map[string]int{user: playcount},
					}
				}
			}
			mu.Unlock()
		}(user)
	}

	wg.Wait()

	if ctx.Err() != nil {
		return nil, errors.NewTimeoutError("request cancelled", ctx.Err())
	}

	usernameLower := strings.ToLower(username)
	var recommendations []types.RecommendedArtist

	for artistKey, agg := range artistStats {
		userPlaycount := agg.UserPlaycounts[usernameLower]
		groupExcludingUser := agg.TotalPlaycount - userPlaycount

		// Only include artists with at least 2 OTHER listeners
		otherListeners := agg.UserCount
		if _, hasUser := agg.UserPlaycounts[usernameLower]; hasUser {
			otherListeners = agg.UserCount - 1
		}
		if otherListeners < 2 {
			continue
		}

		score := float64(groupExcludingUser) / float64(userPlaycount+1)

		// Find canonical name (with original casing)
		canonicalName := artistKey
		for _, user := range c.config.Users {
			if pc, ok := agg.UserPlaycounts[strings.ToLower(user)]; ok && pc > 0 {
				// We stored lowercase keys, so recover the name from first matching user's data
				_ = pc
			}
		}
		_ = canonicalName

		recommendations = append(recommendations, types.RecommendedArtist{
			Name:          artistKey,
			GroupTotal:    groupExcludingUser,
			UserPlaycount: userPlaycount,
			Score:         score,
		})
	}

	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].Score > recommendations[j].Score
	})

	if len(recommendations) > 7 {
		recommendations = recommendations[:7]
	}

	return recommendations, nil
}

// GetHiddenGem returns a user's hidden gems — artists they love that nobody else in the group listens to
func (c *Client) GetHiddenGem(ctx context.Context, username, period string) ([]types.HiddenGemArtist, error) {
	c.logger.Debug("Getting hidden gems", "user", username, "period", period)

	cacheKey := types.CacheKey{
		Type:   "hidden_gem",
		User:   username,
		Period: period,
	}

	result, err := cache.GetOrSet(c.cache, cacheKey, c.config.CacheTTL, func() (types.HiddenGemArtists, error) {
		artists, err := c.fetchHiddenGem(ctx, username, period)
		return types.HiddenGemArtists(artists), err
	})
	if err != nil {
		return nil, errors.NewAPIError(fmt.Sprintf("failed to get hidden gems for %s", username), err)
	}

	return []types.HiddenGemArtist(result), nil
}

// fetchHiddenGem fetches and computes hidden gems for a user
func (c *Client) fetchHiddenGem(ctx context.Context, username, period string) ([]types.HiddenGemArtist, error) {
	// Fetch target user's top artists first
	userArtists, err := c.fetchUserTopArtistsWithPlaycounts(ctx, username, period, 100)
	if err != nil {
		return nil, err
	}

	if len(userArtists) == 0 {
		return nil, nil
	}

	// Build map of user's artists (lowercase name -> playcount)
	userArtistMap := make(map[string]int)
	for _, artist := range userArtists {
		playcount := 0
		if pc, pcErr := strconv.Atoi(artist.PlayCount); pcErr == nil {
			playcount = pc
		}
		userArtistMap[strings.ToLower(artist.Name)] = playcount
	}

	// Fan-out over all OTHER users, accumulate playcounts for artists in userArtistMap
	othersTotal := make(map[string]int)
	othersCount := make(map[string]int)
	var wg sync.WaitGroup
	var mu sync.Mutex
	usernameLower := strings.ToLower(username)

	for _, user := range c.config.Users {
		if strings.ToLower(user) == usernameLower {
			continue
		}

		wg.Add(1)
		go func(user string) {
			defer wg.Done()

			select {
			case c.semaphore <- struct{}{}:
				defer func() { <-c.semaphore }()
			case <-ctx.Done():
				return
			}

			artists, err := c.fetchUserTopArtistsWithPlaycounts(ctx, user, period, 100)
			if err != nil {
				c.logger.Warn("Failed to get top artists for user", "user", user, "error", err)
				return
			}

			mu.Lock()
			for _, artist := range artists {
				key := strings.ToLower(artist.Name)
				if _, inUserMap := userArtistMap[key]; inUserMap {
					playcount := 0
					if pc, pcErr := strconv.Atoi(artist.PlayCount); pcErr == nil {
						playcount = pc
					}
					othersTotal[key] += playcount
					othersCount[key]++
				}
			}
			mu.Unlock()
		}(user)
	}

	wg.Wait()

	if ctx.Err() != nil {
		return nil, errors.NewTimeoutError("request cancelled", ctx.Err())
	}

	var gems []types.HiddenGemArtist
	for _, artist := range userArtists {
		key := strings.ToLower(artist.Name)
		userPlaycount := userArtistMap[key]
		total := othersTotal[key]
		count := othersCount[key]
		score := float64(userPlaycount) / float64(total+1)

		gems = append(gems, types.HiddenGemArtist{
			Name:          artist.Name,
			UserPlaycount: userPlaycount,
			OthersTotal:   total,
			OthersCount:   count,
			Score:         score,
		})
	}

	sort.Slice(gems, func(i, j int) bool {
		return gems[i].Score > gems[j].Score
	})

	if len(gems) > 5 {
		gems = gems[:5]
	}

	return gems, nil
}

// fetchArtistTracksWithUserPlaycounts gets artist tracks and cross-references with user listening data
func (c *Client) fetchArtistTracksWithUserPlaycounts(ctx context.Context, username, artistName string, limit int) ([]string, error) {
	// Get top tracks for this artist
	artistTracksResp, err := c.api.GetArtistTopTracks(ctx, map[string]interface{}{
		"artist": artistName,
		"limit":  limit * 2, // Get more to ensure we have enough after filtering
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get artist tracks: %w", err)
	}

	if len(artistTracksResp.TopTracks.Tracks) == 0 {
		return []string{}, nil
	}

	// Get user's top tracks to cross-reference playcount data
	userTracks, err := c.GetUserTopTracks(ctx, username, "overall", 500)
	if err != nil {
		c.logger.Warn("Failed to get user tracks for cross-reference", "error", err)
		// Continue without playcount data
	}

	// Create a map of user's tracks for quick lookup
	userTrackMap := make(map[string]bool)
	for _, track := range userTracks {
		userTrackMap[strings.ToLower(track)] = true
	}

	// Build result list with tracks the user has listened to
	var result []string
	for _, track := range artistTracksResp.TopTracks.Tracks {
		trackFormatted := formatTrackName(track.Artist.Name, track.Name)

		// Only include tracks the user has actually listened to
		if userTrackMap[strings.ToLower(trackFormatted)] {
			result = append(result, trackFormatted)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}

	return result, nil
}
