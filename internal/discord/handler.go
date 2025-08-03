package discord

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/syakter/go-chuu/internal/charts"
	"github.com/syakter/go-chuu/internal/commands"
	"github.com/syakter/go-chuu/internal/config"
	"github.com/syakter/go-chuu/internal/errors"
	"github.com/syakter/go-chuu/internal/lastfm"
	"github.com/syakter/go-chuu/internal/platform"
	"github.com/syakter/go-chuu/internal/types"
)

// Handler manages Discord interactions
type Handler struct {
	*platform.BaseHandler
	session *discordgo.Session
	config  *config.Config
	logger  *slog.Logger
}

// NewHandler creates a new Discord handler
func NewHandler(
	cfg *config.Config,
	lastfmClient *lastfm.Client,
	chartGen *charts.Generator,
	parser *commands.Parser,
	logger *slog.Logger,
) (*Handler, error) {
	session, err := discordgo.New("Bot " + cfg.DiscordBotToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session: %w", err)
	}

	baseHandler := platform.NewBaseHandler(cfg, lastfmClient, chartGen, parser, logger)

	return &Handler{
		BaseHandler: baseHandler,
		session:     session,
		config:      cfg,
		logger:      logger,
	}, nil
}

// Start begins handling Discord events
func (h *Handler) Start(ctx context.Context) error {
	h.logger.Info("Starting Discord handler")

	// Add handlers
	h.session.AddHandler(h.handleMessageCreate)
	h.session.AddHandler(h.handleReady)

	// Set intents
	h.session.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages

	// Open connection
	if err := h.session.Open(); err != nil {
		return fmt.Errorf("failed to open Discord session: %w", err)
	}

	h.logger.Info("Discord handler started successfully")

	// Wait for context cancellation
	<-ctx.Done()

	h.logger.Info("Discord handler stopping")
	h.session.Close()
	return nil
}

// GetPlatformType returns the platform type
func (h *Handler) GetPlatformType() platform.PlatformType {
	return platform.PlatformDiscord
}

// handleReady handles the ready event
func (h *Handler) handleReady(s *discordgo.Session, event *discordgo.Ready) {
	h.logger.Info("Discord bot is ready", "user", event.User.Username)
}

// handleMessageCreate handles message creation events
func (h *Handler) handleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages from the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Only respond to mentions or direct messages
	if !h.isMentioned(s, m) && m.GuildID != "" {
		return
	}

	start := time.Now()
	h.logger.Info("Processing Discord message",
		"user", m.Author.Username,
		"channel", m.ChannelID,
		"guild", m.GuildID,
		"content", m.Content)

	// Create a timeout context for the command processing
	cmdCtx, cancel := context.WithTimeout(context.Background(), h.config.RequestTimeout)
	defer cancel()

	// Clean the message content (remove mentions)
	cleanContent := h.cleanMessageContent(s, m.Content)

	// Parse the command
	cmd, err := h.Parser.Parse(cleanContent)
	if err != nil {
		h.logger.Warn("Failed to parse command", "error", err, "content", cleanContent)
		h.sendErrorResponse(m.ChannelID, err)
		return
	}

	// Process the command
	response := h.ProcessCommand(cmdCtx, cmd)

	// Send the response
	h.sendResponse(m.ChannelID, response)

	elapsed := time.Since(start)
	h.logger.Info("Completed Discord message processing", "duration", elapsed)
}

// isMentioned checks if the bot was mentioned in the message
func (h *Handler) isMentioned(s *discordgo.Session, m *discordgo.MessageCreate) bool {
	// Check for direct message (no guild)
	if m.GuildID == "" {
		return true
	}

	// Check for bot mention
	for _, user := range m.Mentions {
		if user.ID == s.State.User.ID {
			return true
		}
	}

	return false
}

// cleanMessageContent removes bot mentions from the message content
func (h *Handler) cleanMessageContent(s *discordgo.Session, content string) string {
	// Remove bot mention
	botMention := fmt.Sprintf("<@%s>", s.State.User.ID)
	botMentionNick := fmt.Sprintf("<@!%s>", s.State.User.ID)

	content = strings.Replace(content, botMention, "", -1)
	content = strings.Replace(content, botMentionNick, "", -1)

	return strings.TrimSpace(content)
}

// sendResponse sends a bot response to the specified channel
func (h *Handler) sendResponse(channelID string, response *types.BotResponse) {
	switch response.Type {
	case types.ResponseTypeText:
		if response.Content != "" {
			if _, err := h.session.ChannelMessageSend(channelID, response.Content); err != nil {
				h.logger.Error("Failed to send text message", "error", err)
			}
		}

	case types.ResponseTypeFile:
		if response.File != nil {
			file, err := os.Open(response.File.Path)
			if err != nil {
				h.logger.Error("Failed to open file", "error", err, "path", response.File.Path)
				return
			}
			defer file.Close()

			_, err = h.session.ChannelFileSend(channelID, response.File.Filename, file)
			if err != nil {
				h.logger.Error("Failed to send file", "error", err)
			}
		}

	case types.ResponseTypeError:
		h.sendErrorResponse(channelID, fmt.Errorf(response.Error))
	}
}

// sendErrorResponse sends an error response to the user
func (h *Handler) sendErrorResponse(channelID string, err error) {
	errorMessage := errors.GetUserFriendlyMessage(err)
	if _, sendErr := h.session.ChannelMessageSend(channelID, errorMessage); sendErr != nil {
		h.logger.Error("Failed to send error message", "error", sendErr)
	}
}

// SendTextResponse implements ResponseSender interface
func (h *Handler) SendTextResponse(ctx context.Context, channelID string, content string) error {
	_, err := h.session.ChannelMessageSend(channelID, content)
	return err
}

// SendFileResponse implements ResponseSender interface
func (h *Handler) SendFileResponse(ctx context.Context, channelID string, file *types.FileUpload) error {
	fileHandle, err := os.Open(file.Path)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer fileHandle.Close()

	_, err = h.session.ChannelFileSend(channelID, file.Filename, fileHandle)
	return err
}

// SendErrorResponse implements ResponseSender interface
func (h *Handler) SendErrorResponse(ctx context.Context, channelID string, err error) error {
	errorMessage := errors.GetUserFriendlyMessage(err)
	_, sendErr := h.session.ChannelMessageSend(channelID, errorMessage)
	return sendErr
}
