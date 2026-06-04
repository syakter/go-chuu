package lastfm

import (
	"context"
	"sort"
	"strconv"
	"sync"

	"github.com/syakter/go-chuu/internal/types"
)

// UserRecapEntry holds per-user listening data for a recap period.
type UserRecapEntry struct {
	Username   string
	TopArtists []string // names of top artists by playcount
	TotalPlays int      // sum of top-artist playcounts (proxy for activity)
}

// GroupRecapData holds aggregated group listening data for a recap period.
type GroupRecapData struct {
	Period    string
	Users     []UserRecapEntry   // sorted by TotalPlays descending
	TopAlbums []types.AlbumCount // top 10 albums across the group
	TopTracks []types.TrackCount // top 10 tracks across the group
}

// FetchGroupRecapData gathers per-user and group-wide listening data for the given period.
// Individual user failures are logged and skipped so the recap always returns partial data.
func (c *Client) FetchGroupRecapData(ctx context.Context, period string) (*GroupRecapData, error) {
	if period == "" {
		period = "7d"
	}

	var (
		wg          sync.WaitGroup
		mu          sync.Mutex
		userEntries []UserRecapEntry
	)

	for _, username := range c.config.Users {
		wg.Add(1)
		go func(username string) {
			defer wg.Done()

			select {
			case c.semaphore <- struct{}{}:
				defer func() { <-c.semaphore }()
			case <-ctx.Done():
				return
			}

			artists, err := c.fetchUserTopArtistsWithPlaycounts(ctx, username, period, 8)
			if err != nil {
				c.logger.Warn("Skipping user in recap", "user", username, "error", err)
				return
			}

			entry := UserRecapEntry{Username: username}
			for _, a := range artists {
				entry.TopArtists = append(entry.TopArtists, a.Name)
				if pc, err := strconv.Atoi(a.PlayCount); err == nil {
					entry.TotalPlays += pc
				}
			}

			// Only include users with actual listening activity
			if entry.TotalPlays > 0 {
				mu.Lock()
				userEntries = append(userEntries, entry)
				mu.Unlock()
			}
		}(username)
	}

	wg.Wait()

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	sort.Slice(userEntries, func(i, j int) bool {
		return userEntries[i].TotalPlays > userEntries[j].TotalPlays
	})

	// Reuse cached group methods — failures are non-fatal for the recap
	topAlbums, err := c.GetTopAlbumsAcrossUsers(ctx, period, 10)
	if err != nil {
		c.logger.Warn("Could not fetch group top albums for recap", "error", err)
	}

	topTracks, err := c.GetTopTracksAcrossUsers(ctx, period, 10)
	if err != nil {
		c.logger.Warn("Could not fetch group top tracks for recap", "error", err)
	}

	return &GroupRecapData{
		Period:    period,
		Users:     userEntries,
		TopAlbums: topAlbums,
		TopTracks: topTracks,
	}, nil
}
