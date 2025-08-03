package charts

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/fogleman/gg"
	"github.com/syakter/go-chuu/internal/errors"
	"github.com/syakter/go-chuu/internal/types"
	"github.com/syakter/go-lastfm/lastfm"
)

// Generator handles chart generation
type Generator struct {
	logger     *slog.Logger
	tempDir    string
	httpClient *http.Client
	lastfmAPI  *lastfm.Api
}

// NewGenerator creates a new chart generator
func NewGenerator(logger *slog.Logger, tempDir string, lastfmAPI *lastfm.Api) *Generator {
	return &Generator{
		logger:  logger,
		tempDir: tempDir,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		lastfmAPI: lastfmAPI,
	}
}

// GenerateAlbumChart generates a 3x3 album chart
func (g *Generator) GenerateAlbumChart(ctx context.Context, username, period string) (*types.FileUpload, error) {
	g.logger.Debug("Generating album chart", "username", username, "period", period)

	// Get album data from external API
	albums, err := g.fetchAlbumData(ctx, username, period)
	if err != nil {
		return nil, errors.NewAPIError("failed to fetch album data", err)
	}

	if len(albums) == 0 {
		return nil, errors.NewNotFoundError("no albums found for user in specified period")
	}

	// Create the chart image
	chartImage, err := g.createAlbumChart(ctx, albums)
	if err != nil {
		return nil, errors.NewInternalError("failed to create chart image", err)
	}

	// Save to temporary file
	filename := fmt.Sprintf("%s_album_chart_%s_%d.png", username, period, time.Now().Unix())
	filePath := filepath.Join(g.tempDir, filename)

	file, err := os.Create(filePath)
	if err != nil {
		return nil, errors.NewInternalError("failed to create chart file", err)
	}
	defer file.Close()

	if err := png.Encode(file, chartImage); err != nil {
		return nil, errors.NewInternalError("failed to encode chart image", err)
	}

	// Schedule cleanup
	go g.scheduleCleanup(filePath, 10*time.Minute)

	return &types.FileUpload{
		Filename: filename,
		Path:     filePath,
		Title:    fmt.Sprintf("Album chart for %s (%s)", username, period),
	}, nil
}

// fetchAlbumData fetches album data from the Last.fm API
func (g *Generator) fetchAlbumData(ctx context.Context, username, period string) ([]types.Album, error) {
	// Handle 24h period differently - use recent tracks instead of top albums
	if period == "24h" {
		return g.fetchAlbumsFromRecentTracks(ctx, username)
	}

	formattedPeriod := g.formatPeriodForAPI(period)

	result, err := g.lastfmAPI.User.GetTopAlbums(lastfm.P{
		"user":   username,
		"period": formattedPeriod,
		"limit":  "9", // We only need 9 albums for 3x3 grid
	})
	if err != nil {
		// Check for common Last.fm API errors
		if lastfmErr, ok := err.(*lastfm.LastfmError); ok {
			switch lastfmErr.Code {
			case 6: // User not found
				return nil, fmt.Errorf("Last.fm user '%s' not found", username)
			case 26: // API key suspended
				return nil, fmt.Errorf("Last.fm API key suspended or invalid")
			case 29: // Rate limit exceeded
				return nil, fmt.Errorf("Last.fm API rate limit exceeded, please try again later")
			default:
				return nil, fmt.Errorf("Last.fm API error: %s", lastfmErr.Message)
			}
		}
		return nil, fmt.Errorf("failed to get top albums from Last.fm: %w", err)
	}

	var albums []types.Album
	for _, album := range result.Albums {
		// Find the largest image URL
		imageURL := ""
		for _, img := range album.Images {
			if img.Size == "large" || img.Size == "extralarge" {
				imageURL = img.Url
				break
			}
		}
		// Fallback to any available image
		if imageURL == "" && len(album.Images) > 0 {
			imageURL = album.Images[len(album.Images)-1].Url
		}

		albums = append(albums, types.Album{
			Name:   album.Name,
			Artist: album.Artist.Name,
			Image:  imageURL,
		})
	}

	return albums, nil
}

// fetchAlbumsFromRecentTracks fetches albums from recent tracks in the past 24 hours
func (g *Generator) fetchAlbumsFromRecentTracks(ctx context.Context, username string) ([]types.Album, error) {
	g.logger.Debug("Fetching albums from recent tracks for 24h period", "username", username)

	// Calculate 24 hours ago timestamp
	twentyFourHoursAgo := time.Now().Add(-24 * time.Hour)
	fromTimestamp := strconv.FormatInt(twentyFourHoursAgo.Unix(), 10)

	// We'll need to paginate through recent tracks to get enough data
	albumCounts := make(map[string]*albumInfo)
	page := 1
	const tracksPerPage = 200 // Max allowed by Last.fm API
	const maxPages = 10       // Limit to prevent infinite loops

	for page <= maxPages {
		result, err := g.lastfmAPI.User.GetRecentTracks(lastfm.P{
			"user":  username,
			"limit": strconv.Itoa(tracksPerPage),
			"page":  strconv.Itoa(page),
			"from":  fromTimestamp,
		})
		if err != nil {
			// Check for common Last.fm API errors
			if lastfmErr, ok := err.(*lastfm.LastfmError); ok {
				switch lastfmErr.Code {
				case 6: // User not found
					return nil, fmt.Errorf("Last.fm user '%s' not found", username)
				case 26: // API key suspended
					return nil, fmt.Errorf("Last.fm API key suspended or invalid")
				case 29: // Rate limit exceeded
					return nil, fmt.Errorf("Last.fm API rate limit exceeded, please try again later")
				default:
					return nil, fmt.Errorf("Last.fm API error: %s", lastfmErr.Message)
				}
			}
			return nil, fmt.Errorf("failed to get recent tracks from Last.fm: %w", err)
		}

		if len(result.Tracks) == 0 {
			break // No more tracks
		}

		// Process tracks and aggregate by album
		for _, track := range result.Tracks {
			// Skip tracks without album information
			if track.Album.Name == "" || track.Artist.Name == "" {
				continue
			}

			// Skip now playing tracks (they don't have timestamps)
			if track.NowPlaying == "true" {
				continue
			}

			// Check if track is within 24 hours (additional safety check)
			if track.Date.Uts != "" {
				uts, err := strconv.ParseInt(track.Date.Uts, 10, 64)
				if err == nil {
					trackTime := time.Unix(uts, 0)
					if trackTime.Before(twentyFourHoursAgo) {
						continue // Track is older than 24 hours
					}
				}
			}

			// Create album key (artist + album name for uniqueness)
			albumKey := track.Artist.Name + " - " + track.Album.Name

			if albumData, exists := albumCounts[albumKey]; exists {
				albumData.playCount++
			} else {
				// Find the largest image URL
				imageURL := ""
				for _, img := range track.Images {
					if img.Size == "large" || img.Size == "extralarge" {
						imageURL = img.Url
						break
					}
				}
				// Fallback to any available image
				if imageURL == "" && len(track.Images) > 0 {
					imageURL = track.Images[len(track.Images)-1].Url
				}

				albumCounts[albumKey] = &albumInfo{
					album: types.Album{
						Name:   track.Album.Name,
						Artist: track.Artist.Name,
						Image:  imageURL,
					},
					playCount: 1,
				}
			}
		}

		// If we got fewer tracks than requested, we've reached the end
		if len(result.Tracks) < tracksPerPage {
			break
		}

		page++
	}

	if len(albumCounts) == 0 {
		return nil, fmt.Errorf("no albums found in the past 24 hours for user '%s'", username)
	}

	// Convert map to slice and sort by play count
	var albumList []albumInfo
	for _, albumData := range albumCounts {
		albumList = append(albumList, *albumData)
	}

	// Sort by play count (descending)
	sort.Slice(albumList, func(i, j int) bool {
		return albumList[i].playCount > albumList[j].playCount
	})

	// Take top 9 albums for 3x3 grid
	var albums []types.Album
	limit := 9
	if len(albumList) < limit {
		limit = len(albumList)
	}

	for i := 0; i < limit; i++ {
		albums = append(albums, albumList[i].album)
	}

	g.logger.Debug("Found albums from recent tracks", "count", len(albums), "total_unique_albums", len(albumList))
	return albums, nil
}

// albumInfo holds album data with play count for sorting
type albumInfo struct {
	album     types.Album
	playCount int
}

// createAlbumChart creates a 3x3 chart image from album data
func (g *Generator) createAlbumChart(ctx context.Context, albums []types.Album) (image.Image, error) {
	const (
		width  = 900
		height = 900
		rows   = 3
		cols   = 3
	)

	dc := gg.NewContext(width, height)
	dc.SetRGB(0, 0, 0) // Black background
	dc.Clear()

	albumWidth := width / cols
	albumHeight := height / rows

	// Limit to 9 albums for 3x3 grid
	maxAlbums := 9
	if len(albums) < maxAlbums {
		maxAlbums = len(albums)
	}

	for i := 0; i < maxAlbums; i++ {
		album := albums[i]
		x := float64(i%cols) * (float64(width) / float64(cols))
		y := float64(i/cols) * (float64(height) / float64(rows))

		// Download and process album art
		img, err := g.downloadAlbumArt(ctx, album.Image)
		if err != nil {
			g.logger.Warn("Failed to download album art", "album", album.Name, "artist", album.Artist, "error", err)
			// Draw placeholder rectangle
			dc.SetRGB(0.3, 0.3, 0.3)
			dc.DrawRectangle(x, y, float64(albumWidth), float64(albumHeight))
			dc.Fill()
		} else {
			// Resize to fit grid cell
			resizedImg := imaging.Resize(img, albumWidth, albumHeight, imaging.Lanczos)
			// Draw album art
			dc.DrawImage(resizedImg, int(x), int(y))
		}

		// Add text overlay
		g.drawTextOverlay(dc, album, x, y, float64(albumWidth), float64(albumHeight))
	}

	return dc.Image(), nil
}

// downloadAlbumArt downloads album artwork from URL
func (g *Generator) downloadAlbumArt(ctx context.Context, imageURL string) (image.Image, error) {
	if imageURL == "" {
		return nil, fmt.Errorf("empty image URL")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", imageURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download image: status %d", resp.StatusCode)
	}

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, err
	}

	return img, nil
}

// drawTextOverlay draws album and artist text over the album cover
func (g *Generator) drawTextOverlay(dc *gg.Context, album types.Album, x, y, width, height float64) {
	// Set up text styling - load font with proper error handling
	g.loadFont(dc)

	// Prepare text content
	artistText := g.truncateText(album.Artist, 25)
	albumText := g.truncateText(album.Name, 25)

	// Calculate text position (top left corner of the album square)
	textX := x + 8  // Small padding from left edge
	textY := y + 15 // Small padding from top edge

	// Draw semi-transparent background for text readability
	textBgHeight := 40.0       // Reduced height
	textBgWidth := width * 0.6 // Reduced width to be less intrusive
	dc.SetRGBA(0, 0, 0, 0.6)   // Slightly more transparent
	dc.DrawRectangle(x, y, textBgWidth, textBgHeight)
	dc.Fill()                                             // Draw artist name (upper line)
	dc.SetRGB(1, 1, 1)                                    // White text
	dc.DrawStringAnchored(artistText, textX, textY, 0, 0) // Left-aligned

	// Draw album name (lower line)
	dc.SetRGB(0.9, 0.9, 0.9)                                // Slightly dimmer white
	dc.DrawStringAnchored(albumText, textX, textY+18, 0, 0) // Left-aligned, 18px below artist (tighter spacing)
}

// truncateText truncates text to fit within specified length
func (g *Generator) truncateText(text string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}

	// Find a good place to cut (prefer word boundaries)
	words := strings.Fields(text)
	result := ""

	for _, word := range words {
		if len(result+" "+word) > maxLength-3 { // Reserve space for "..."
			break
		}
		if result != "" {
			result += " "
		}
		result += word
	}

	if result == "" {
		// If even the first word is too long, truncate it
		result = text[:maxLength-3]
	}

	return result + "..."
}

// loadFont attempts to load a suitable font for text rendering
func (g *Generator) loadFont(dc *gg.Context) {
	// Try loading fonts in order of preference
	fontPaths := []string{
		"/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/TTF/DejaVuSans-Bold.ttf",
		"/System/Library/Fonts/Arial.ttf",
		"/System/Library/Fonts/Helvetica.ttc",
	}

	for _, fontPath := range fontPaths {
		if err := dc.LoadFontFace(fontPath, 16); err == nil {
			// Font loaded successfully, no need to log in normal operation
			return
		}
	}

	// Only log a warning if no fonts could be loaded at all
	g.logger.Warn("No system fonts available, using built-in default font")
}

// formatPeriodForAPI formats period for the Last.fm API
func (g *Generator) formatPeriodForAPI(period string) string {
	switch period {
	case "24h":
		return "24h" // Special case - handled by fetchAlbumsFromRecentTracks
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

// scheduleCleanup removes temporary files after a delay
func (g *Generator) scheduleCleanup(filePath string, delay time.Duration) {
	time.Sleep(delay)
	if err := os.Remove(filePath); err != nil {
		g.logger.Warn("Failed to clean up temporary file", "path", filePath, "error", err)
	} else {
		g.logger.Debug("Cleaned up temporary file", "path", filePath)
	}
}

// EnsureTempDir creates the temporary directory if it doesn't exist
func (g *Generator) EnsureTempDir() error {
	return os.MkdirAll(g.tempDir, 0755)
}
