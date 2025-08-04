package help

import (
	"strings"
	"testing"
)

func TestFormatter_FormatSlack(t *testing.T) {
	formatter := NewFormatter(PlatformSlack)
	content := &HelpContent{
		Title: "Test Bot Commands",
		Sections: []Section{
			{
				Title: "Basic Commands",
				Icon:  "🎵",
				Commands: []Command{
					{
						Name:        "test",
						Usage:       "!test <arg>",
						Description: "A test command",
						Example:     "!test hello",
					},
				},
			},
		},
		Footer: "Test footer",
	}

	result := formatter.Format(content)

	// Check that Slack formatting is applied
	if !strings.Contains(result, "*Test Bot Commands*") {
		t.Error("Expected Slack title formatting (*title*) not found")
	}

	if !strings.Contains(result, "*🎵 Basic Commands*") {
		t.Error("Expected Slack section formatting not found")
	}

	if !strings.Contains(result, "`!test <arg>`") {
		t.Error("Expected Slack code formatting not found")
	}

	if !strings.Contains(result, "_Example:_ `!test hello`") {
		t.Error("Expected Slack example formatting not found")
	}

	if !strings.Contains(result, "_Test footer_") {
		t.Error("Expected Slack footer formatting not found")
	}
}

func TestFormatter_FormatDiscord(t *testing.T) {
	formatter := NewFormatter(PlatformDiscord)
	content := &HelpContent{
		Title: "Test Bot Commands",
		Sections: []Section{
			{
				Title: "Basic Commands",
				Icon:  "🎵",
				Commands: []Command{
					{
						Name:        "test",
						Usage:       "!test <arg>",
						Description: "A test command",
						Example:     "!test hello",
					},
				},
			},
		},
		Footer: "Test footer",
	}

	result := formatter.Format(content)

	// Check that Discord formatting is applied
	if !strings.Contains(result, "# Test Bot Commands") {
		t.Error("Expected Discord title formatting (# title) not found")
	}

	if !strings.Contains(result, "## 🎵 Basic Commands") {
		t.Error("Expected Discord section formatting not found")
	}

	if !strings.Contains(result, "`!test <arg>`") {
		t.Error("Expected Discord code formatting not found")
	}

	if !strings.Contains(result, "*Example:* `!test hello`") {
		t.Error("Expected Discord example formatting not found")
	}

	if !strings.Contains(result, "*Test footer*") {
		t.Error("Expected Discord footer formatting not found")
	}
}

func TestFormatter_FormatGeneric(t *testing.T) {
	formatter := NewFormatter(PlatformGeneric)
	content := &HelpContent{
		Title: "Test Bot Commands",
		Sections: []Section{
			{
				Title: "Basic Commands",
				Commands: []Command{
					{
						Name:        "test",
						Usage:       "!test <arg>",
						Description: "A test command",
						Example:     "!test hello",
					},
				},
			},
		},
		Footer: "Test footer",
	}

	result := formatter.Format(content)

	// Check that generic formatting is applied
	if !strings.Contains(result, "=== Test Bot Commands ===") {
		t.Error("Expected generic title formatting not found")
	}

	if !strings.Contains(result, "Basic Commands:") {
		t.Error("Expected generic section formatting not found")
	}

	if !strings.Contains(result, "  !test <arg>") {
		t.Error("Expected generic command formatting not found")
	}

	if !strings.Contains(result, "    Example: !test hello") {
		t.Error("Expected generic example formatting not found")
	}
}

func TestGetHelpContent(t *testing.T) {
	content := GetHelpContent()

	if content.Title == "" {
		t.Error("Help content should have a title")
	}

	if len(content.Sections) == 0 {
		t.Error("Help content should have sections")
	}

	// Check that listening club section exists
	found := false
	for _, section := range content.Sections {
		if strings.Contains(section.Title, "Listening Club") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Help content should include Listening Club section")
	}

	if content.Footer == "" {
		t.Error("Help content should have a footer")
	}
}
