package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/syakter/go-chuu/internal/cache"
	"github.com/syakter/go-chuu/internal/charts"
	"github.com/syakter/go-chuu/internal/commands"
	"github.com/syakter/go-chuu/internal/config"
	"github.com/syakter/go-chuu/internal/errors"
	"github.com/syakter/go-chuu/internal/lastfm"
	"github.com/syakter/go-chuu/internal/logger"
	"github.com/syakter/go-chuu/internal/types"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Load .env if present (same as bot; ignore missing file)
	godotenv.Load() //nolint:errcheck

	cfg, err := config.LoadForCLI()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Logger writes to stderr so it doesn't pollute stdout output
	log := logger.New(logger.Config{
		Format: logger.FormatPretty,
		Level:  slog.LevelError,
		Output: os.Stderr,
	})

	botCache := cache.NewInMemoryCache(1000)
	lastfmClient := lastfm.NewClient(cfg, botCache, log)

	tempDir := filepath.Join(os.TempDir(), "go-chuu-charts")
	chartGen := charts.NewGenerator(log, tempDir, lastfmClient)
	if err := chartGen.EnsureTempDir(); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	parser := commands.NewParser(cfg.Users)

	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Print(commands.GetHelpText())
		return nil
	}

	// Auto-prepend ! to known command names so users can write `go-chuu-cli top samin`
	knownCmds := map[string]bool{
		"help": true, "up": true, "uptime": true, "chart": true, "np": true,
		"track": true, "top": true, "ta": true, "topartist": true, "rp": true,
		"leaderboard": true, "artist": true, "kga": true, "kgt": true,
		"disco": true, "dt": true, "t": true, "rec": true, "hidden": true,
	}
	if knownCmds[strings.ToLower(args[0])] {
		args[0] = "!" + args[0]
	}

	message := strings.Join(args, " ")

	cmd, err := parser.Parse(message)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.RequestTimeout)
	defer cancel()

	return dispatch(ctx, cmd, lastfmClient, chartGen)
}

func dispatch(ctx context.Context, cmd *types.Command, lf *lastfm.Client, chartGen *charts.Generator) error {
	switch cmd.Type {
	case types.CommandHelp:
		fmt.Print(commands.GetHelpText())

	case types.CommandUptime:
		fmt.Printf("go-chuu CLI — started at %s\n", time.Now().Format(time.RFC3339))

	case types.CommandChart:
		return handleChart(ctx, cmd, chartGen)

	case types.CommandNowPlaying:
		return handleNowPlaying(ctx, lf)

	case types.CommandArtistFans:
		return handleArtistFans(ctx, cmd, lf)

	case types.CommandAlbumFans:
		return handleAlbumFans(ctx, cmd, lf)

	case types.CommandTrackFans:
		return handleTrackFans(ctx, cmd, lf)

	case types.CommandLeaderboard:
		return handleLeaderboard(ctx, lf)

	case types.CommandTopTracks:
		return handleTopTracks(ctx, cmd, lf)

	case types.CommandTopAlbums:
		return handleTopAlbums(ctx, cmd, lf)

	case types.CommandTopArtists:
		return handleTopArtists(ctx, cmd, lf)

	case types.CommandRecentTracks:
		return handleRecentTracks(ctx, cmd, lf)

	case types.CommandTopAlbumsAll:
		return handleTopAlbumsAll(ctx, cmd, lf)

	case types.CommandTopTracksAll:
		return handleTopTracksAll(ctx, cmd, lf)

	case types.CommandDisco:
		return handleDisco(ctx, cmd, lf)

	case types.CommandDiscoveryTrack:
		return handleDiscoveryTrack(ctx, cmd, lf)

	case types.CommandRecommend:
		return handleRecommend(ctx, cmd, lf)

	case types.CommandHiddenGem:
		return handleHiddenGem(ctx, cmd, lf)

	default:
		fmt.Fprintf(os.Stderr, "Command not implemented\n")
		os.Exit(1)
	}

	return nil
}

// --- command handlers ---

func handleChart(ctx context.Context, cmd *types.Command, chartGen *charts.Generator) error {
	fileUpload, err := chartGen.GenerateAlbumChart(ctx, cmd.User, cmd.Period, cmd.ChartSize)
	if err != nil {
		return fmt.Errorf("%s", errors.GetUserFriendlyMessage(err))
	}

	fmt.Printf("Chart saved: %s\n", fileUpload.Path)

	// Try to open with OS viewer
	openCmd := "open"
	if runtime.GOOS == "linux" {
		openCmd = "xdg-open"
	} else if runtime.GOOS != "darwin" {
		// Unsupported OS — skip open attempt
		return nil
	}
	exec.Command(openCmd, fileUpload.Path).Start() //nolint:errcheck

	return nil
}

func handleNowPlaying(ctx context.Context, lf *lastfm.Client) error {
	nowPlaying, err := lf.GetNowPlaying(ctx)
	if err != nil {
		return fmt.Errorf("%s", errors.GetUserFriendlyMessage(err))
	}

	if len(nowPlaying) == 0 {
		fmt.Println("Nobody is listening to anything right now!")
		return nil
	}

	fmt.Println("What everyone is listening to right now:")
	fmt.Println()
	for user, track := range nowPlaying {
		fmt.Printf("%s is listening to %s\n", user, track)
	}

	return nil
}

func handleArtistFans(ctx context.Context, cmd *types.Command, lf *lastfm.Client) error {
	userCounts, err := lf.GetArtistScrobbles(ctx, cmd.Artist)
	if err != nil {
		return fmt.Errorf("%s", errors.GetUserFriendlyMessage(err))
	}

	printUserCounts(fmt.Sprintf("Top %s fans in Kagang:", cmd.Artist), userCounts)
	return nil
}

func handleAlbumFans(ctx context.Context, cmd *types.Command, lf *lastfm.Client) error {
	userCounts, err := lf.GetAlbumScrobbles(ctx, cmd.Artist, cmd.Album)
	if err != nil {
		return fmt.Errorf("%s", errors.GetUserFriendlyMessage(err))
	}

	printUserCounts(fmt.Sprintf("Top %s - %s fans in Kagang:", cmd.Artist, cmd.Album), userCounts)
	return nil
}

func handleTrackFans(ctx context.Context, cmd *types.Command, lf *lastfm.Client) error {
	userCounts, err := lf.GetTrackScrobbles(ctx, cmd.Artist, cmd.Track)
	if err != nil {
		return fmt.Errorf("%s", errors.GetUserFriendlyMessage(err))
	}

	printUserCounts(fmt.Sprintf("Top %s - %s fans in Kagang:", cmd.Artist, cmd.Track), userCounts)
	return nil
}

func handleLeaderboard(ctx context.Context, lf *lastfm.Client) error {
	leaderboard, err := lf.GetWeeklyLeaderboard(ctx)
	if err != nil {
		return fmt.Errorf("%s", errors.GetUserFriendlyMessage(err))
	}

	if len(leaderboard) > 0 {
		from := leaderboard[0].PeriodFrom.Format("2006/01/02")
		to := leaderboard[0].PeriodTo.Format("2006/01/02")
		fmt.Printf("Weekly Leaderboard (%s to %s):\n\n", from, to)
	} else {
		fmt.Println("Weekly Leaderboard:")
		fmt.Println()
	}

	for _, entry := range leaderboard {
		var prefix string
		switch entry.Rank {
		case 1:
			prefix = "1."
		case 2:
			prefix = "2."
		case 3:
			prefix = "3."
		default:
			prefix = fmt.Sprintf("%d.", entry.Rank)
		}
		fmt.Printf("%s %s: %d scrobbles\n", prefix, entry.Username, entry.Scrobbles)
	}

	return nil
}

func handleTopTracks(ctx context.Context, cmd *types.Command, lf *lastfm.Client) error {
	tracks, err := lf.GetUserTopTracks(ctx, cmd.User, cmd.Period, 10)
	if err != nil {
		return fmt.Errorf("%s", errors.GetUserFriendlyMessage(err))
	}

	fmt.Printf("%s's top tracks%s:\n\n", cmd.User, periodText(cmd.Period))
	for i, track := range tracks {
		fmt.Printf("%d. %s\n", i+1, track)
	}

	return nil
}

func handleTopAlbums(ctx context.Context, cmd *types.Command, lf *lastfm.Client) error {
	albums, err := lf.GetUserTopAlbums(ctx, cmd.User, cmd.Period, 10)
	if err != nil {
		return fmt.Errorf("%s", errors.GetUserFriendlyMessage(err))
	}

	fmt.Printf("%s's top albums%s:\n\n", cmd.User, periodText(cmd.Period))
	for i, album := range albums {
		fmt.Printf("%d. %s\n", i+1, album)
	}

	return nil
}

func handleTopArtists(ctx context.Context, cmd *types.Command, lf *lastfm.Client) error {
	artists, err := lf.GetUserTopArtists(ctx, cmd.User, cmd.Period, 10)
	if err != nil {
		return fmt.Errorf("%s", errors.GetUserFriendlyMessage(err))
	}

	fmt.Printf("%s's top artists%s:\n\n", cmd.User, periodText(cmd.Period))
	for i, artist := range artists {
		fmt.Printf("%d. %s\n", i+1, artist)
	}

	return nil
}

func handleRecentTracks(ctx context.Context, cmd *types.Command, lf *lastfm.Client) error {
	tracks, err := lf.GetUserRecentTracks(ctx, cmd.User, cmd.Limit)
	if err != nil {
		return fmt.Errorf("%s", errors.GetUserFriendlyMessage(err))
	}

	fmt.Printf("%s's recent tracks:\n\n", cmd.User)
	for i, track := range tracks {
		fmt.Printf("%d. %s\n", i+1, track)
	}

	return nil
}

func handleTopAlbumsAll(ctx context.Context, cmd *types.Command, lf *lastfm.Client) error {
	period := cmd.Period
	if period == "" {
		period = "7d"
	}

	albums, err := lf.GetTopAlbumsAcrossUsers(ctx, period, 10)
	if err != nil {
		return fmt.Errorf("%s", errors.GetUserFriendlyMessage(err))
	}

	if len(albums) == 0 {
		fmt.Println("No albums found for the specified period!")
		return nil
	}

	fmt.Printf("Top albums in Kagang%s:\n\n", periodText(period))
	for i, album := range albums {
		fmt.Printf("%d. %s (%d scrobbles, %d users)\n", i+1, album.AlbumName, album.Playcount, album.UserCount)
	}

	return nil
}

func handleTopTracksAll(ctx context.Context, cmd *types.Command, lf *lastfm.Client) error {
	period := cmd.Period
	if period == "" {
		period = "7d"
	}

	tracks, err := lf.GetTopTracksAcrossUsers(ctx, period, 10)
	if err != nil {
		return fmt.Errorf("%s", errors.GetUserFriendlyMessage(err))
	}

	if len(tracks) == 0 {
		fmt.Println("No tracks found for the specified period!")
		return nil
	}

	fmt.Printf("Top tracks in Kagang%s:\n\n", periodText(period))
	for i, track := range tracks {
		fmt.Printf("%d. %s (%d scrobbles, %d users)\n", i+1, track.TrackName, track.Playcount, track.UserCount)
	}

	return nil
}

func handleDisco(ctx context.Context, cmd *types.Command, lf *lastfm.Client) error {
	albums, err := lf.GetUserTopAlbumsByArtist(ctx, cmd.User, cmd.Artist, 10)
	if err != nil {
		return fmt.Errorf("%s", errors.GetUserFriendlyMessage(err))
	}

	if len(albums) == 0 {
		fmt.Printf("No albums by %s found for %s!\n", cmd.Artist, cmd.User)
		return nil
	}

	fmt.Printf("%s's top albums by %s:\n\n", cmd.User, cmd.Artist)
	for i, album := range albums {
		fmt.Printf("%d. %s\n", i+1, album)
	}

	return nil
}

func handleDiscoveryTrack(ctx context.Context, cmd *types.Command, lf *lastfm.Client) error {
	tracks, err := lf.GetUserTopTracksByArtist(ctx, cmd.User, cmd.Artist, 10)
	if err != nil {
		return fmt.Errorf("%s", errors.GetUserFriendlyMessage(err))
	}

	if len(tracks) == 0 {
		fmt.Printf("No tracks by %s found for %s!\n", cmd.Artist, cmd.User)
		return nil
	}

	fmt.Printf("%s's top tracks by %s:\n\n", cmd.User, cmd.Artist)
	for i, track := range tracks {
		fmt.Printf("%d. %s\n", i+1, track)
	}

	return nil
}

func handleRecommend(ctx context.Context, cmd *types.Command, lf *lastfm.Client) error {
	period := cmd.Period
	if period == "" {
		period = "overall"
	}

	recs, err := lf.GetGroupRecommendations(ctx, cmd.User, period)
	if err != nil {
		return fmt.Errorf("%s", errors.GetUserFriendlyMessage(err))
	}

	if len(recs) == 0 {
		fmt.Printf("No recommendations found for %s — they might already listen to everything the group loves!\n", cmd.User)
		return nil
	}

	fmt.Printf("Artists the group loves that %s should check out%s:\n\n", cmd.User, periodText(period))
	for i, rec := range recs {
		if rec.UserPlaycount == 0 {
			fmt.Printf("%d. %s — %d group scrobbles (0 plays by %s)\n", i+1, rec.Name, rec.GroupTotal, cmd.User)
		} else {
			fmt.Printf("%d. %s — %d group scrobbles (%d plays by %s)\n", i+1, rec.Name, rec.GroupTotal, rec.UserPlaycount, cmd.User)
		}
	}

	return nil
}

func handleHiddenGem(ctx context.Context, cmd *types.Command, lf *lastfm.Client) error {
	period := cmd.Period
	if period == "" {
		period = "overall"
	}

	gems, err := lf.GetHiddenGem(ctx, cmd.User, period)
	if err != nil {
		return fmt.Errorf("%s", errors.GetUserFriendlyMessage(err))
	}

	if len(gems) == 0 {
		fmt.Printf("No hidden gems found for %s!\n", cmd.User)
		return nil
	}

	fmt.Printf("%s's hidden gems%s:\n\n", cmd.User, periodText(period))
	for i, gem := range gems {
		var othersDesc string
		switch gem.OthersCount {
		case 0:
			othersDesc = "nobody else also listens"
		case 1:
			othersDesc = "1 other person also listens"
		default:
			othersDesc = fmt.Sprintf("%d others also listen", gem.OthersCount)
		}
		fmt.Printf("%d. %s — %d plays (%s)\n", i+1, gem.Name, gem.UserPlaycount, othersDesc)
	}

	return nil
}

// --- helpers ---

func printUserCounts(title string, userCounts []types.UserCount) {
	fmt.Println(title)
	fmt.Println()
	for i, uc := range userCounts {
		var prefix string
		switch i {
		case 0:
			prefix = "1."
		case 1:
			prefix = "2."
		case 2:
			prefix = "3."
		default:
			prefix = fmt.Sprintf("%d.", i+1)
		}
		fmt.Printf("%s %s: %d scrobbles\n", prefix, uc.Username, uc.Playcount)
	}
}

func periodText(period string) string {
	if period == "" || period == "overall" {
		return " of all time"
	}
	switch period {
	case "7d", "1w":
		return " for the past 7 days"
	case "1m", "30d":
		return " for the past month"
	case "3m", "90d":
		return " for the past 3 months"
	case "6m", "180d":
		return " for the past 6 months"
	case "1y", "365d":
		return " for the past year"
	default:
		return fmt.Sprintf(" for period: %s", period)
	}
}
