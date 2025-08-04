package slack

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"github.com/syakter/go-chuu/internal/charts"
	"github.com/syakter/go-chuu/internal/commands"
	"github.com/syakter/go-chuu/internal/config"
	"github.com/syakter/go-chuu/internal/errors"
	"github.com/syakter/go-chuu/internal/lastfm"
	"github.com/syakter/go-chuu/internal/platform"
	"github.com/syakter/go-chuu/internal/types"
)

// Handler manages Slack interactions
type Handler struct {
	*platform.BaseHandler
	api    *slack.Client
	client *socketmode.Client
	config *config.Config
	logger *slog.Logger
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
	baseHandler := platform.NewBaseHandler(cfg, lastfmClient, chartGen, parser, logger)

	return &Handler{
		BaseHandler: baseHandler,
		api:         api,
		client:      client,
		config:      cfg,
		logger:      logger,
	}
}

// Start begins handling Slack events
func (h *Handler) Start(ctx context.Context) error {
	h.logger.Info("Starting Slack handler")

	go h.handleEvents(ctx)

	return h.client.RunContext(ctx)
}

// GetPlatformType returns the platform type
func (h *Handler) GetPlatformType() platform.PlatformType {
	return platform.PlatformSlack
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
	cmd, err := h.Parser.Parse(event.Text)
	if err != nil {
		h.logger.Warn("Failed to parse command", "error", err, "text", event.Text)
		h.sendErrorResponse(event.Channel, err)
		return
	}

	// Process the command
	response := h.processCommand(cmdCtx, cmd, event.User)

	// Send the response
	h.sendResponse(event.Channel, response)

	elapsed := time.Since(start)
	h.logger.Info("Completed app mention processing", "duration", elapsed)
}

// processCommand processes a parsed command and returns a response
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

// SendTextResponse implements ResponseSender interface
func (h *Handler) SendTextResponse(ctx context.Context, channelID string, content string) error {
	_, _, err := h.api.PostMessage(channelID, slack.MsgOptionText(content, false))
	return err
}

// SendFileResponse implements ResponseSender interface
func (h *Handler) SendFileResponse(ctx context.Context, channelID string, file *types.FileUpload) error {
	params := slack.FileUploadParameters{
		File:     file.Path,
		Filename: file.Filename,
		Channels: []string{channelID},
		Title:    file.Title,
	}

	_, err := h.api.UploadFile(params)
	return err
}

// SendErrorResponse implements ResponseSender interface
func (h *Handler) SendErrorResponse(ctx context.Context, channelID string, err error) error {
	errorMessage := errors.GetUserFriendlyMessage(err)
	_, _, sendErr := h.api.PostMessage(channelID, slack.MsgOptionText(errorMessage, false))
	return sendErr
}

// processCommand wraps the base ProcessCommand with Slack-specific user context
func (h *Handler) processCommand(ctx context.Context, cmd *types.Command, userID string) *types.BotResponse {
	// Handle platform-specific commands
	switch cmd.Type {
	case types.CommandHelp:
		return &types.BotResponse{
			Type:    types.ResponseTypeText,
			Content: commands.GetFormattedHelpText("slack"),
		}
	case types.CommandLCVote:
		return h.handleLCVote(ctx, cmd, userID, "Unknown User")
	}

	// For all other commands, use the base handler
	return h.ProcessCommand(ctx, cmd)
}

// handleLCVote handles listening club voting with Slack user context
func (h *Handler) handleLCVote(ctx context.Context, cmd *types.Command, userID, username string) *types.BotResponse {
	if cmd.Rating < 1 || cmd.Rating > 10 {
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: "Rating must be between 1 and 10",
		}
	}

	// Use the listening club service with proper user context
	if err := h.ListeningClub.Vote("slack", userID, username, cmd.Rating, cmd.Comment); err != nil {
		h.logger.Error("Failed to record vote", "error", err)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: "Failed to record vote: " + err.Error(),
		}
	}

	response := fmt.Sprintf("✅ Your vote of %d/10 has been recorded!", cmd.Rating)
	if cmd.Comment != "" {
		response += fmt.Sprintf("\nComment: %s", cmd.Comment)
	}

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: response,
	}
}
