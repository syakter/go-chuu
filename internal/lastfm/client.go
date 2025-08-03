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
	"github.com/syakter/go-lastfm/lastfm"
)

// Client wraps the Last.fm API with enhanced functionality
type Client struct {
	api                   *lastfm.Api
	cache                 cache.Cache
	config                *config.Config
	logger                *slog.Logger
	maxConcurrentRequests int
	semaphore             chan struct{}
}

// NewClient creates a new Last.fm client
func NewClient(cfg *config.Config, cache cache.Cache, logger *slog.Logger) *Client {
	api := lastfm.New(cfg.LastFMAPIKey, cfg.LastFMAPISecret)

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
func (c *Client) GetAPI() *lastfm.Api {
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

			result, err := c.api.Artist.GetInfo(lastfm.P{"artist": artistName, "username": user})
			if err != nil {
				c.logger.Warn("Failed to get artist info", "user", user, "artist", artistName, "error", err)
				// Don't fail the entire request for one user
				mu.Lock()
				counts[user] = 0
				mu.Unlock()
				return
			}

			playcount := 0
			if result.Stats.UserPlays != "" {
				if pc, err := strconv.Atoi(result.Stats.UserPlays); err == nil {
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

			result, err := c.api.Album.GetInfo(lastfm.P{"artist": artistName, "album": albumName, "username": user})
			if err != nil {
				c.logger.Warn("Failed to get album info", "user", user, "artist", artistName, "album", albumName, "error", err)
				mu.Lock()
				counts[user] = 0
				mu.Unlock()
				return
			}

			playcount := 0
			if result.UserPlayCount != "" {
				if pc, err := strconv.Atoi(result.UserPlayCount); err == nil {
					playcount = pc
				}
			}

			mu.Lock()
			counts[user] = playcount
			mu.Unlock()
		}(user)
	}

	wg.Wait()

	if ctx.Err() != nil {
		return nil, errors.NewTimeoutError("request cancelled", ctx.Err())
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

			result, err := c.api.Track.GetInfo(lastfm.P{"artist": artistName, "track": trackName, "username": user})
			if err != nil {
				c.logger.Warn("Failed to get track info", "user", user, "artist", artistName, "track", trackName, "error", err)
				mu.Lock()
				counts[user] = 0
				mu.Unlock()
				return
			}

			playcount := 0
			if result.UserPlayCount != "" {
				if pc, err := strconv.Atoi(result.UserPlayCount); err == nil {
					playcount = pc
				}
			}

			mu.Lock()
			counts[user] = playcount
			mu.Unlock()
		}(user)
	}

	wg.Wait()

	if ctx.Err() != nil {
		return nil, errors.NewTimeoutError("request cancelled", ctx.Err())
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

			result, err := c.api.User.GetRecentTracks(lastfm.P{"user": user, "limit": 1})
			if err != nil {
				c.logger.Warn("Failed to get recent tracks", "user", user, "error", err)
				return
			}

			if len(result.Tracks) > 0 {
				track := result.Tracks[0]
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

	result, err := c.api.User.GetTopAlbums(lastfm.P{"user": username, "period": period, "limit": limit})
	if err != nil {
		return nil, err
	}

	var albums []string
	for i, album := range result.Albums {
		if i >= limit {
			break
		}
		albums = append(albums, fmt.Sprintf("%s - %s", album.Artist.Name, album.Name))
	}

	return albums, nil
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

	result, err := c.api.User.GetTopArtists(lastfm.P{"user": username, "period": period, "limit": limit})
	if err != nil {
		return nil, err
	}

	var artists []string
	for i, artist := range result.Artists {
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
	result, err := c.api.User.GetRecentTracks(lastfm.P{"user": username, "limit": limit})
	if err != nil {
		return nil, err
	}

	var tracks []string
	for i, track := range result.Tracks {
		if i >= limit {
			break
		}
		// Skip now playing tracks in recent tracks list
		if track.NowPlaying == "true" {
			continue
		}
		tracks = append(tracks, fmt.Sprintf("%s - %s", track.Artist.Name, track.Name))
	}

	return tracks, nil
}

// fetchUserTopTracks fetches user top tracks from the API
func (c *Client) fetchUserTopTracks(ctx context.Context, username, period string, limit int) ([]string, error) {
	period = c.normalizePeriod(period)

	result, err := c.api.User.GetTopTracks(lastfm.P{"user": username, "period": period, "limit": limit})
	if err != nil {
		return nil, err
	}

	var tracks []string
	for i, track := range result.Tracks {
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

			fromTimestamp := strconv.FormatInt(fromTime.Unix(), 10)
			toTimestamp := strconv.FormatInt(toTime.Unix(), 10)

			artistChart, err := c.api.User.GetWeeklyArtistChart(lastfm.P{"user": user, "from": fromTimestamp, "to": toTimestamp})
			if err != nil {
				c.logger.Warn("Failed to get weekly artist chart", "user", user, "error", err)
				return
			}

			totalPlayCount := 0
			for _, artist := range artistChart.Artists {
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

	if ctx.Err() != nil {
		return nil, errors.NewTimeoutError("request cancelled", ctx.Err())
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
