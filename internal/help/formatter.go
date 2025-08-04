package help

import (
	"fmt"
	"strings"
)

// Platform represents different chat platforms
type Platform string

const (
	PlatformSlack   Platform = "slack"
	PlatformDiscord Platform = "discord"
	PlatformGeneric Platform = "generic"
)

// Command represents a bot command for help documentation
type Command struct {
	Name        string
	Usage       string
	Description string
	Example     string
}

// Section represents a group of related commands
type Section struct {
	Title    string
	Icon     string
	Commands []Command
}

// HelpContent contains all the help information
type HelpContent struct {
	Title    string
	Sections []Section
	Footer   string
}

// Formatter handles platform-specific formatting of help content
type Formatter struct {
	platform Platform
}

// NewFormatter creates a new help formatter for the specified platform
func NewFormatter(platform Platform) *Formatter {
	return &Formatter{platform: platform}
}

// Format renders the help content with platform-specific formatting
func (f *Formatter) Format(content *HelpContent) string {
	var result strings.Builder

	// Title
	result.WriteString(f.formatTitle(content.Title))
	result.WriteString("\n\n")

	// Sections
	for i, section := range content.Sections {
		if i > 0 {
			result.WriteString("\n")
		}
		result.WriteString(f.formatSection(section))
	}

	// Footer
	if content.Footer != "" {
		result.WriteString("\n")
		result.WriteString(f.formatFooter(content.Footer))
	}

	return result.String()
}

// formatTitle formats the main title
func (f *Formatter) formatTitle(title string) string {
	switch f.platform {
	case PlatformSlack:
		return fmt.Sprintf("*%s*", title)
	case PlatformDiscord:
		return fmt.Sprintf("# %s", title)
	default:
		return fmt.Sprintf("=== %s ===", title)
	}
}

// formatSection formats a section with its commands
func (f *Formatter) formatSection(section Section) string {
	var result strings.Builder

	// Section header
	header := section.Title
	if section.Icon != "" {
		header = section.Icon + " " + header
	}

	switch f.platform {
	case PlatformSlack:
		result.WriteString(fmt.Sprintf("*%s*\n", header))
	case PlatformDiscord:
		result.WriteString(fmt.Sprintf("## %s\n", header))
	default:
		result.WriteString(fmt.Sprintf("%s:\n", header))
	}

	// Commands
	for _, cmd := range section.Commands {
		result.WriteString(f.formatCommand(cmd))
	}

	return result.String()
}

// formatCommand formats an individual command
func (f *Formatter) formatCommand(cmd Command) string {
	switch f.platform {
	case PlatformSlack:
		return f.formatSlackCommand(cmd)
	case PlatformDiscord:
		return f.formatDiscordCommand(cmd)
	default:
		return f.formatGenericCommand(cmd)
	}
}

// formatSlackCommand formats a command for Slack
func (f *Formatter) formatSlackCommand(cmd Command) string {
	var result strings.Builder

	// Command usage in code block
	result.WriteString(fmt.Sprintf("`%s`", cmd.Usage))

	// Description
	if cmd.Description != "" {
		result.WriteString(fmt.Sprintf(" - %s", cmd.Description))
	}

	result.WriteString("\n")

	// Example if provided
	if cmd.Example != "" {
		result.WriteString(fmt.Sprintf("    _Example:_ `%s`\n", cmd.Example))
	}

	return result.String()
}

// formatDiscordCommand formats a command for Discord
func (f *Formatter) formatDiscordCommand(cmd Command) string {
	var result strings.Builder

	// Command usage in code block
	result.WriteString(fmt.Sprintf("`%s`", cmd.Usage))

	// Description
	if cmd.Description != "" {
		result.WriteString(fmt.Sprintf(" - %s", cmd.Description))
	}

	result.WriteString("\n")

	// Example if provided
	if cmd.Example != "" {
		result.WriteString(fmt.Sprintf("  *Example:* `%s`\n", cmd.Example))
	}

	return result.String()
}

// formatGenericCommand formats a command for generic text output
func (f *Formatter) formatGenericCommand(cmd Command) string {
	var result strings.Builder

	result.WriteString(fmt.Sprintf("  %s", cmd.Usage))

	if cmd.Description != "" {
		result.WriteString(fmt.Sprintf(" - %s", cmd.Description))
	}

	result.WriteString("\n")

	if cmd.Example != "" {
		result.WriteString(fmt.Sprintf("    Example: %s\n", cmd.Example))
	}

	return result.String()
}

// formatFooter formats the footer text
func (f *Formatter) formatFooter(footer string) string {
	switch f.platform {
	case PlatformSlack:
		return fmt.Sprintf("_%s_", footer)
	case PlatformDiscord:
		return fmt.Sprintf("*%s*", footer)
	default:
		return footer
	}
}
