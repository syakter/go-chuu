package slack

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"github.com/syakter/go-chuu/internal/charts"
	"github.com/syakter/go-chuu/internal/commands"
	"github.com/syakter/go-chuu/internal/config"
	"github.com/syakter/go-chuu/internal/errors"
	"github.com/syakter/go-chuu/internal/lastfm"
	"github.com/syakter/go-chuu/internal/types"
)

// Handler manages Slack interactions
type Handler struct {
	api          *slack.Client
	client       *socketmode.Client
	lastfmClient *lastfm.Client
	chartGen     *charts.Generator
	parser       *commands.Parser
	config       *config.Config
	logger       *slog.Logger
	startTime    time.Time
}

// NewHandler creates a new Slack handler
func NewHandler(
	cfg *config.Config,
	lastfmClient *lastfm.Client,
	chartGen *charts.Generator,
	parser *commands.Parser,
	logger *slog.Logger,
) *Handler {
	api := slack.New(
		cfg.SlackBotToken,
		slack.OptionAppLevelToken(cfg.SlackAppToken),
	)

	client := socketmode.New(api)

	return &Handler{
		api:          api,
		client:       client,
		lastfmClient: lastfmClient,
		chartGen:     chartGen,
		parser:       parser,
		config:       cfg,
		logger:       logger,
		startTime:    time.Now(),
	}
}

// Start begins handling Slack events
func (h *Handler) Start(ctx context.Context) error {
	h.logger.Info("Starting Slack handler")

	go h.handleEvents(ctx)

	return h.client.RunContext(ctx)
}

// handleEvents processes incoming Slack events
func (h *Handler) handleEvents(ctx context.Context) {
	for {
		select {
		case evt := <-h.client.Events:
			h.processEvent(ctx, evt)
		case <-ctx.Done():
			h.logger.Info("Event handler stopping")
			return
		}
	}
}

// processEvent processes individual Slack events
func (h *Handler) processEvent(ctx context.Context, evt socketmode.Event) {
	switch evt.Type {
	case socketmode.EventTypeEventsAPI:
		h.handleEventsAPI(ctx, evt)
	case socketmode.EventTypeConnecting:
		h.logger.Info("Connecting to Slack...")
	case socketmode.EventTypeConnected:
		h.logger.Info("Connected to Slack")
	case socketmode.EventTypeHello:
		h.logger.Debug("Received hello event from Slack")
	default:
		h.logger.Debug("Unexpected event type", "type", evt.Type)
	}
}

// handleEventsAPI handles Events API events
func (h *Handler) handleEventsAPI(ctx context.Context, evt socketmode.Event) {
	eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
	if !ok {
		h.logger.Warn("Invalid Events API event", "event", evt)
		return
	}

	h.logger.Debug("Received Events API event", "type", eventsAPIEvent.Type)

	// Acknowledge the event
	h.client.Ack(*evt.Request)

	switch eventsAPIEvent.Type {
	case slackevents.CallbackEvent:
		h.handleCallbackEvent(ctx, eventsAPIEvent.InnerEvent)
	default:
		h.logger.Debug("Unsupported Events API event type", "type", eventsAPIEvent.Type)
	}
}

// handleCallbackEvent handles callback events (mentions, messages)
func (h *Handler) handleCallbackEvent(ctx context.Context, innerEvent slackevents.EventsAPIInnerEvent) {
	switch ev := innerEvent.Data.(type) {
	case *slackevents.AppMentionEvent:
		h.handleAppMention(ctx, ev)
	default:
		h.logger.Debug("Unsupported callback event", "type", fmt.Sprintf("%T", ev))
	}
}

// handleAppMention processes app mention events
func (h *Handler) handleAppMention(ctx context.Context, event *slackevents.AppMentionEvent) {
	start := time.Now()
	h.logger.Info("Processing app mention", "user", event.User, "channel", event.Channel, "text", event.Text)

	// Create a timeout context for the command processing
	cmdCtx, cancel := context.WithTimeout(ctx, h.config.RequestTimeout)
	defer cancel()

	// Parse the command
	cmd, err := h.parser.Parse(event.Text)
	if err != nil {
		h.logger.Warn("Failed to parse command", "error", err, "text", event.Text)
		h.sendErrorResponse(event.Channel, err)
		return
	}

	// Process the command
	response := h.processCommand(cmdCtx, cmd)

	// Send the response
	h.sendResponse(event.Channel, response)

	elapsed := time.Since(start)
	h.logger.Info("Completed app mention processing", "duration", elapsed)
}

// processCommand processes a parsed command and returns a response
func (h *Handler) processCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	h.logger.Debug("Processing command", "type", cmd.Type, "user", cmd.User)

	switch cmd.Type {
	case types.CommandHelp:
		return &types.BotResponse{
			Type:    types.ResponseTypeText,
			Content: commands.GetHelpText(),
		}

	case types.CommandUptime:
		uptime := time.Since(h.startTime)
		return &types.BotResponse{
			Type:    types.ResponseTypeText,
			Content: fmt.Sprintf("Uptime: %s", uptime.String()),
		}

	case types.CommandChart:
		return h.handleChartCommand(ctx, cmd)

	case types.CommandNowPlaying:
		return h.handleNowPlayingCommand(ctx)

	case types.CommandArtistFans:
		return h.handleArtistFansCommand(ctx, cmd)

	case types.CommandAlbumFans:
		return h.handleAlbumFansCommand(ctx, cmd)

	case types.CommandTrackFans:
		return h.handleTrackFansCommand(ctx, cmd)

	case types.CommandLeaderboard:
		return h.handleLeaderboardCommand(ctx)

	case types.CommandTopTracks:
		return h.handleTopTracksCommand(ctx, cmd)

	case types.CommandTopAlbums:
		return h.handleTopAlbumsCommand(ctx, cmd)

	case types.CommandTopArtists:
		return h.handleTopArtistsCommand(ctx, cmd)

	case types.CommandRecentTracks:
		return h.handleRecentTracksCommand(ctx, cmd)

	case types.CommandTopAlbumsAll:
		return h.handleTopAlbumsAllCommand(ctx, cmd)

	case types.CommandTopTracksAll:
		return h.handleTopTracksAllCommand(ctx, cmd)

	case types.CommandDisco:
		return h.handleDiscoCommand(ctx, cmd)

	case types.CommandDiscoveryTrack:
		return h.handleDiscoveryTrackCommand(ctx, cmd)

	default:
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: "Command not implemented yet",
		}
	}
}

// handleChartCommand processes chart generation commands
func (h *Handler) handleChartCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	fileUpload, err := h.chartGen.GenerateAlbumChart(ctx, cmd.User, cmd.Period)
	if err != nil {
		h.logger.Error("Failed to generate chart", "error", err, "user", cmd.User, "period", cmd.Period)
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
func (h *Handler) handleNowPlayingCommand(ctx context.Context) *types.BotResponse {
	nowPlaying, err := h.lastfmClient.GetNowPlaying(ctx)
	if err != nil {
		h.logger.Error("Failed to get now playing", "error", err)
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
func (h *Handler) handleArtistFansCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	userCounts, err := h.lastfmClient.GetArtistScrobbles(ctx, cmd.Artist)
	if err != nil {
		h.logger.Error("Failed to get artist scrobbles", "error", err, "artist", cmd.Artist)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	content := h.formatUserCounts(fmt.Sprintf("Top %s fans in Kagang:", cmd.Artist), userCounts)

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content,
	}
}

// handleAlbumFansCommand processes album fans commands
func (h *Handler) handleAlbumFansCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	userCounts, err := h.lastfmClient.GetAlbumScrobbles(ctx, cmd.Artist, cmd.Album)
	if err != nil {
		h.logger.Error("Failed to get album scrobbles", "error", err, "artist", cmd.Artist, "album", cmd.Album)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	content := h.formatUserCounts(fmt.Sprintf("Top %s - %s fans in Kagang:", cmd.Artist, cmd.Album), userCounts)

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content,
	}
}

// handleTrackFansCommand processes track fans commands
func (h *Handler) handleTrackFansCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	userCounts, err := h.lastfmClient.GetTrackScrobbles(ctx, cmd.Artist, cmd.Track)
	if err != nil {
		h.logger.Error("Failed to get track scrobbles", "error", err, "artist", cmd.Artist, "track", cmd.Track)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	content := h.formatUserCounts(fmt.Sprintf("Top %s - %s fans in Kagang:", cmd.Artist, cmd.Track), userCounts)

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content,
	}
}

// handleLeaderboardCommand processes leaderboard commands
func (h *Handler) handleLeaderboardCommand(ctx context.Context) *types.BotResponse {
	leaderboard, err := h.lastfmClient.GetWeeklyLeaderboard(ctx)
	if err != nil {
		h.logger.Error("Failed to get weekly leaderboard", "error", err)
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
func (h *Handler) handleTopTracksCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	tracks, err := h.lastfmClient.GetUserTopTracks(ctx, cmd.User, cmd.Period, 10)
	if err != nil {
		h.logger.Error("Failed to get user top tracks", "error", err, "user", cmd.User, "period", cmd.Period)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	var content strings.Builder
	periodText := h.formatPeriodText(cmd.Period)
	content.WriteString(fmt.Sprintf("%s's top tracks%s:\n\n", cmd.User, periodText))

	for i, track := range tracks {
		content.WriteString(fmt.Sprintf("%d. %s\n", i+1, track))
	}

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content.String(),
	}
}

// formatUserCounts formats user counts into a readable string
func (h *Handler) formatUserCounts(title string, userCounts []types.UserCount) string {
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
func (h *Handler) formatPeriodText(period string) string {
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

// sendResponse sends a bot response to the specified channel
func (h *Handler) sendResponse(channel string, response *types.BotResponse) {
	switch response.Type {
	case types.ResponseTypeText:
		if response.Content != "" {
			if _, _, err := h.api.PostMessage(channel, slack.MsgOptionText(response.Content, false)); err != nil {
				h.logger.Error("Failed to send text message", "error", err)
			}
		}

	case types.ResponseTypeFile:
		if response.File != nil {
			params := slack.FileUploadParameters{
				File:     response.File.Path,
				Filename: response.File.Filename,
				Channels: []string{channel},
				Title:    response.File.Title,
			}

			if _, err := h.api.UploadFile(params); err != nil {
				h.logger.Error("Failed to upload file", "error", err)
			}
		}

	case types.ResponseTypeError:
		h.sendErrorResponse(channel, fmt.Errorf(response.Error))
	}
}

// sendErrorResponse sends an error response to the user
func (h *Handler) sendErrorResponse(channel string, err error) {
	errorMessage := errors.GetUserFriendlyMessage(err)
	if _, _, sendErr := h.api.PostMessage(channel, slack.MsgOptionText(errorMessage, false)); sendErr != nil {
		h.logger.Error("Failed to send error message", "error", sendErr)
	}
}

// handleTopAlbumsCommand processes top albums commands
func (h *Handler) handleTopAlbumsCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	albums, err := h.lastfmClient.GetUserTopAlbums(ctx, cmd.User, cmd.Period, 10)
	if err != nil {
		h.logger.Error("Failed to get user top albums", "error", err, "user", cmd.User, "period", cmd.Period)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	var content strings.Builder
	periodText := h.formatPeriodText(cmd.Period)
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
func (h *Handler) handleTopArtistsCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	artists, err := h.lastfmClient.GetUserTopArtists(ctx, cmd.User, cmd.Period, 10)
	if err != nil {
		h.logger.Error("Failed to get user top artists", "error", err, "user", cmd.User, "period", cmd.Period)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	var content strings.Builder
	periodText := h.formatPeriodText(cmd.Period)
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
func (h *Handler) handleRecentTracksCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	tracks, err := h.lastfmClient.GetUserRecentTracks(ctx, cmd.User, cmd.Limit)
	if err != nil {
		h.logger.Error("Failed to get user recent tracks", "error", err, "user", cmd.User, "limit", cmd.Limit)
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
func (h *Handler) handleTopAlbumsAllCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	// This would require aggregating data across all users - placeholder for now
	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: "Top albums for all users feature coming soon! 🎵",
	}
}

// handleTopTracksAllCommand processes top tracks for all users commands
func (h *Handler) handleTopTracksAllCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	// This would require aggregating data across all users - placeholder for now
	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: "Top tracks for all users feature coming soon! 🎵",
	}
}

// handleDiscoCommand processes disco commands
func (h *Handler) handleDiscoCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	// This would show top albums by a specific artist for a user - placeholder for now
	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: fmt.Sprintf("Discovery feature for %s by %s coming soon! 🎵", cmd.Artist, cmd.User),
	}
}

// handleDiscoveryTrackCommand processes discovery track commands
func (h *Handler) handleDiscoveryTrackCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	// This would show top tracks by a specific artist for a user - placeholder for now
	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: fmt.Sprintf("Track discovery for %s by %s coming soon! 🎵", cmd.Artist, cmd.User),
	}
}
