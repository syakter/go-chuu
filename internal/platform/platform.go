package platform

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/syakter/go-chuu/internal/charts"
	"github.com/syakter/go-chuu/internal/commands"
	"github.com/syakter/go-chuu/internal/config"
	"github.com/syakter/go-chuu/internal/errors"
	"github.com/syakter/go-chuu/internal/lastfm"
	"github.com/syakter/go-chuu/internal/types"
)

// PlatformType represents different bot platforms
type PlatformType string

const (
	PlatformSlack   PlatformType = "slack"
	PlatformDiscord PlatformType = "discord"
)

// Handler represents a platform-specific bot handler
type Handler interface {
	// Start begins handling platform events
	Start(ctx context.Context) error
	// GetPlatformType returns the platform type
	GetPlatformType() PlatformType
}

// MessageContext contains information about where a message came from
type MessageContext struct {
	ChannelID string
	UserID    string
	GuildID   string // Discord-specific, empty for Slack
	Text      string
	Platform  PlatformType
}

// ResponseSender handles sending responses back to the platform
type ResponseSender interface {
	// SendTextResponse sends a text message
	SendTextResponse(ctx context.Context, channelID string, content string) error
	// SendFileResponse sends a file
	SendFileResponse(ctx context.Context, channelID string, file *types.FileUpload) error
	// SendErrorResponse sends an error message
	SendErrorResponse(ctx context.Context, channelID string, err error) error
}

// BaseHandler provides common functionality for all platform handlers
type BaseHandler struct {
	lastfmClient *lastfm.Client
	chartGen     *charts.Generator
	Parser       *commands.Parser
	config       *config.Config
	logger       *slog.Logger
	startTime    time.Time
}

// NewBaseHandler creates a new base handler with shared dependencies
func NewBaseHandler(
	cfg *config.Config,
	lastfmClient *lastfm.Client,
	chartGen *charts.Generator,
	parser *commands.Parser,
	logger *slog.Logger,
) *BaseHandler {
	return &BaseHandler{
		lastfmClient: lastfmClient,
		chartGen:     chartGen,
		Parser:       parser,
		config:       cfg,
		logger:       logger,
		startTime:    time.Now(),
	}
}

// ProcessCommand processes a parsed command and returns a response
// This is shared logic that can be used by all platform handlers
func (b *BaseHandler) ProcessCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	b.logger.Debug("Processing command", "type", cmd.Type, "user", cmd.User)

	switch cmd.Type {
	case types.CommandHelp:
		return &types.BotResponse{
			Type:    types.ResponseTypeText,
			Content: commands.GetHelpText(),
		}

	case types.CommandUptime:
		uptime := time.Since(b.startTime)
		return &types.BotResponse{
			Type:    types.ResponseTypeText,
			Content: fmt.Sprintf("Uptime: %s", uptime.String()),
		}

	case types.CommandChart:
		return b.handleChartCommand(ctx, cmd)

	case types.CommandNowPlaying:
		return b.handleNowPlayingCommand(ctx)

	case types.CommandArtistFans:
		return b.handleArtistFansCommand(ctx, cmd)

	case types.CommandAlbumFans:
		return b.handleAlbumFansCommand(ctx, cmd)

	case types.CommandTrackFans:
		return b.handleTrackFansCommand(ctx, cmd)

	case types.CommandLeaderboard:
		return b.handleLeaderboardCommand(ctx)

	case types.CommandTopTracks:
		return b.handleTopTracksCommand(ctx, cmd)

	case types.CommandTopAlbums:
		return b.handleTopAlbumsCommand(ctx, cmd)

	case types.CommandTopArtists:
		return b.handleTopArtistsCommand(ctx, cmd)

	case types.CommandRecentTracks:
		return b.handleRecentTracksCommand(ctx, cmd)

	case types.CommandTopAlbumsAll:
		return b.handleTopAlbumsAllCommand(ctx, cmd)

	case types.CommandTopTracksAll:
		return b.handleTopTracksAllCommand(ctx, cmd)

	case types.CommandDisco:
		return b.handleDiscoCommand(ctx, cmd)

	case types.CommandDiscoveryTrack:
		return b.handleDiscoveryTrackCommand(ctx, cmd)

	default:
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: "Command not implemented yet",
		}
	}
}

// handleChartCommand processes chart generation commands
func (b *BaseHandler) handleChartCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	fileUpload, err := b.chartGen.GenerateAlbumChart(ctx, cmd.User, cmd.Period)
	if err != nil {
		b.logger.Error("Failed to generate chart", "error", err, "user", cmd.User, "period", cmd.Period)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	return &types.BotResponse{
		Type: types.ResponseTypeFile,
		File: fileUpload,
	}
}

// handleNowPlayingCommand processes now playing commands
func (b *BaseHandler) handleNowPlayingCommand(ctx context.Context) *types.BotResponse {
	nowPlaying, err := b.lastfmClient.GetNowPlaying(ctx)
	if err != nil {
		b.logger.Error("Failed to get now playing", "error", err)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	if len(nowPlaying) == 0 {
		return &types.BotResponse{
			Type:    types.ResponseTypeText,
			Content: "Nobody is listening to anything right now! 🎵",
		}
	}

	var content strings.Builder
	content.WriteString("What everyone is listening to right now:\n\n")

	for user, track := range nowPlaying {
		content.WriteString(fmt.Sprintf("%s is listening to %s\n", user, track))
	}

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content.String(),
	}
}

// handleArtistFansCommand processes artist fans commands
func (b *BaseHandler) handleArtistFansCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	userCounts, err := b.lastfmClient.GetArtistScrobbles(ctx, cmd.Artist)
	if err != nil {
		b.logger.Error("Failed to get artist scrobbles", "error", err, "artist", cmd.Artist)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	content := b.formatUserCounts(fmt.Sprintf("Top %s fans in Kagang:", cmd.Artist), userCounts)

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content,
	}
}

// handleAlbumFansCommand processes album fans commands
func (b *BaseHandler) handleAlbumFansCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	userCounts, err := b.lastfmClient.GetAlbumScrobbles(ctx, cmd.Artist, cmd.Album)
	if err != nil {
		b.logger.Error("Failed to get album scrobbles", "error", err, "artist", cmd.Artist, "album", cmd.Album)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	content := b.formatUserCounts(fmt.Sprintf("Top %s - %s fans in Kagang:", cmd.Artist, cmd.Album), userCounts)

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content,
	}
}

// handleTrackFansCommand processes track fans commands
func (b *BaseHandler) handleTrackFansCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	userCounts, err := b.lastfmClient.GetTrackScrobbles(ctx, cmd.Artist, cmd.Track)
	if err != nil {
		b.logger.Error("Failed to get track scrobbles", "error", err, "artist", cmd.Artist, "track", cmd.Track)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	content := b.formatUserCounts(fmt.Sprintf("Top %s - %s fans in Kagang:", cmd.Artist, cmd.Track), userCounts)

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content,
	}
}

// handleLeaderboardCommand processes leaderboard commands
func (b *BaseHandler) handleLeaderboardCommand(ctx context.Context) *types.BotResponse {
	leaderboard, err := b.lastfmClient.GetWeeklyLeaderboard(ctx)
	if err != nil {
		b.logger.Error("Failed to get weekly leaderboard", "error", err)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	var content strings.Builder
	if len(leaderboard) > 0 {
		fromDate := leaderboard[0].PeriodFrom.Format("2006/01/02")
		toDate := leaderboard[0].PeriodTo.Format("2006/01/02")
		content.WriteString(fmt.Sprintf("Weekly Leaderboard (%s to %s):\n\n", fromDate, toDate))
	} else {
		content.WriteString("Weekly Leaderboard:\n\n")
	}

	for _, entry := range leaderboard {
		var emoji string
		switch entry.Rank {
		case 1:
			emoji = "👑"
		case 2:
			emoji = "🥈"
		case 3:
			emoji = "🥉"
		default:
			emoji = fmt.Sprintf("%d.", entry.Rank)
		}

		content.WriteString(fmt.Sprintf("%s %s: %d scrobbles\n", emoji, entry.Username, entry.Scrobbles))
	}

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content.String(),
	}
}

// handleTopTracksCommand processes top tracks commands
func (b *BaseHandler) handleTopTracksCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	tracks, err := b.lastfmClient.GetUserTopTracks(ctx, cmd.User, cmd.Period, 10)
	if err != nil {
		b.logger.Error("Failed to get user top tracks", "error", err, "user", cmd.User, "period", cmd.Period)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	var content strings.Builder
	periodText := b.formatPeriodText(cmd.Period)
	content.WriteString(fmt.Sprintf("%s's top tracks%s:\n\n", cmd.User, periodText))

	for i, track := range tracks {
		content.WriteString(fmt.Sprintf("%d. %s\n", i+1, track))
	}

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content.String(),
	}
}

// handleTopAlbumsCommand processes top albums commands
func (b *BaseHandler) handleTopAlbumsCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	albums, err := b.lastfmClient.GetUserTopAlbums(ctx, cmd.User, cmd.Period, 10)
	if err != nil {
		b.logger.Error("Failed to get user top albums", "error", err, "user", cmd.User, "period", cmd.Period)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	var content strings.Builder
	periodText := b.formatPeriodText(cmd.Period)
	content.WriteString(fmt.Sprintf("%s's top albums%s:\n\n", cmd.User, periodText))

	for i, album := range albums {
		content.WriteString(fmt.Sprintf("%d. %s\n", i+1, album))
	}

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content.String(),
	}
}

// handleTopArtistsCommand processes top artists commands
func (b *BaseHandler) handleTopArtistsCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	artists, err := b.lastfmClient.GetUserTopArtists(ctx, cmd.User, cmd.Period, 10)
	if err != nil {
		b.logger.Error("Failed to get user top artists", "error", err, "user", cmd.User, "period", cmd.Period)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	var content strings.Builder
	periodText := b.formatPeriodText(cmd.Period)
	content.WriteString(fmt.Sprintf("%s's top artists%s:\n\n", cmd.User, periodText))

	for i, artist := range artists {
		content.WriteString(fmt.Sprintf("%d. %s\n", i+1, artist))
	}

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content.String(),
	}
}

// handleRecentTracksCommand processes recent tracks commands
func (b *BaseHandler) handleRecentTracksCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	tracks, err := b.lastfmClient.GetUserRecentTracks(ctx, cmd.User, cmd.Limit)
	if err != nil {
		b.logger.Error("Failed to get user recent tracks", "error", err, "user", cmd.User, "limit", cmd.Limit)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	var content strings.Builder
	content.WriteString(fmt.Sprintf("%s's recent tracks:\n\n", cmd.User))

	for i, track := range tracks {
		content.WriteString(fmt.Sprintf("%d. %s\n", i+1, track))
	}

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content.String(),
	}
}

// handleTopAlbumsAllCommand processes top albums for all users commands
func (b *BaseHandler) handleTopAlbumsAllCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	// This would require aggregating data across all users - placeholder for now
	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: "Top albums for all users feature coming soon! 🎵",
	}
}

// handleTopTracksAllCommand processes top tracks for all users commands
func (b *BaseHandler) handleTopTracksAllCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	// This would require aggregating data across all users - placeholder for now
	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: "Top tracks for all users feature coming soon! 🎵",
	}
}

// handleDiscoCommand processes disco commands
func (b *BaseHandler) handleDiscoCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	// This would show top albums by a specific artist for a user - placeholder for now
	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: fmt.Sprintf("Discovery feature for %s by %s coming soon! 🎵", cmd.Artist, cmd.User),
	}
}

// handleDiscoveryTrackCommand processes discovery track commands
func (b *BaseHandler) handleDiscoveryTrackCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	// This would show top tracks by a specific artist for a user - placeholder for now
	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: fmt.Sprintf("Track discovery for %s by %s coming soon! 🎵", cmd.Artist, cmd.User),
	}
}

// formatUserCounts formats user counts into a readable string
func (b *BaseHandler) formatUserCounts(title string, userCounts []types.UserCount) string {
	var content strings.Builder
	content.WriteString(title + "\n\n")

	for i, userCount := range userCounts {
		var prefix string
		switch i {
		case 0:
			prefix = "👑."
		case 1:
			prefix = "🥈."
		case 2:
			prefix = "🥉."
		default:
			prefix = fmt.Sprintf("%d.", i+1)
		}

		content.WriteString(fmt.Sprintf("%s %s: %d scrobbles\n", prefix, userCount.Username, userCount.Playcount))
	}

	return content.String()
}

// formatPeriodText formats period into readable text
func (b *BaseHandler) formatPeriodText(period string) string {
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
