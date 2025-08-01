package charts

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/disintegration/imaging"
	"github.com/fogleman/gg"
	"github.com/syakter/go-chuu/internal/errors"
	"github.com/syakter/go-chuu/internal/types"
)

// Generator handles chart generation
type Generator struct {
	logger     *slog.Logger
	tempDir    string
	httpClient *http.Client
}

// NewGenerator creates a new chart generator
func NewGenerator(logger *slog.Logger, tempDir string) *Generator {
	return &Generator{
		logger:  logger,
		tempDir: tempDir,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
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

// fetchAlbumData fetches album data from the external topster.gg API
func (g *Generator) fetchAlbumData(ctx context.Context, username, period string) ([]types.Album, error) {
	formattedPeriod := g.formatPeriodForAPI(period)
	url := fmt.Sprintf("http://topster.gg/api/topalbums/%s/%s", username, formattedPeriod)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var albums []types.Album
	if err := json.Unmarshal(body, &albums); err != nil {
		return nil, err
	}

	return albums, nil
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
			continue
		}

		// Resize to fit grid cell
		resizedImg := imaging.Resize(img, albumWidth, albumHeight, imaging.Lanczos)

		// Draw album art
		dc.DrawImage(resizedImg, int(x), int(y))
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

// formatPeriodForAPI formats period for the topster.gg API
func (g *Generator) formatPeriodForAPI(period string) string {
	switch period {
	case "7d", "1w":
		return "7day"
	case "1d":
		return "1day"
	case "30d", "1m":
		return "30day"
	case "1y":
		return "365day"
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
