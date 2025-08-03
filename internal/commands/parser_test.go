package commands

import (
	"testing"

	"github.com/syakter/go-chuu/internal/types"
)

func TestParser_Parse(t *testing.T) {
	validUsers := []string{"user1", "user2", "testuser"}
	parser := NewParser(validUsers)

	tests := []struct {
		name     string
		input    string
		expected *types.Command
		wantErr  bool
	}{
		{
			name:  "help command",
			input: "!help",
			expected: &types.Command{
				Type:     types.CommandHelp,
				RawInput: "!help",
			},
			wantErr: false,
		},
		{
			name:  "uptime command",
			input: "!up",
			expected: &types.Command{
				Type:     types.CommandUptime,
				RawInput: "!up",
			},
			wantErr: false,
		},
		{
			name:  "chart command with user and period",
			input: "!chart user1 7d",
			expected: &types.Command{
				Type:     types.CommandChart,
				User:     "user1",
				Period:   "7d",
				RawInput: "!chart user1 7d",
			},
			wantErr: false,
		},
		{
			name:  "chart command with 24h period",
			input: "!chart user1 24h",
			expected: &types.Command{
				Type:     types.CommandChart,
				User:     "user1",
				Period:   "24h",
				RawInput: "!chart user1 24h",
			},
			wantErr: false,
		},
		{
			name:  "top tracks command",
			input: "!track testuser 1m",
			expected: &types.Command{
				Type:     types.CommandTopTracks,
				User:     "testuser",
				Period:   "1m",
				RawInput: "!track testuser 1m",
			},
			wantErr: false,
		},
		{
			name:  "artist query",
			input: "Radiohead",
			expected: &types.Command{
				Type:     types.CommandArtistFans,
				Artist:   "Radiohead",
				RawInput: "Radiohead",
			},
			wantErr: false,
		},
		{
			name:  "album by artist query",
			input: "OK Computer by Radiohead",
			expected: &types.Command{
				Type:     types.CommandAlbumFans,
				Album:    "OK Computer",
				Artist:   "Radiohead",
				RawInput: "OK Computer by Radiohead",
			},
			wantErr: false,
		},
		{
			name:  "album with 'by' in name",
			input: "Songs by Tony Sly by No Use for a Name",
			expected: &types.Command{
				Type:     types.CommandAlbumFans,
				Album:    "Songs by Tony Sly",
				Artist:   "No Use for a Name",
				RawInput: "Songs by Tony Sly by No Use for a Name",
			},
			wantErr: false,
		},
		{
			name:  "album with multiple 'by' words",
			input: "Made by Walking by Kings of Leon",
			expected: &types.Command{
				Type:     types.CommandAlbumFans,
				Album:    "Made by Walking",
				Artist:   "Kings of Leon",
				RawInput: "Made by Walking by Kings of Leon",
			},
			wantErr: false,
		},
		{
			name:  "album with capitalized By",
			input: "Led by the Heart By Nikka Costa",
			expected: &types.Command{
				Type:     types.CommandAlbumFans,
				Album:    "Led by the Heart",
				Artist:   "Nikka Costa",
				RawInput: "Led by the Heart By Nikka Costa",
			},
			wantErr: false,
		},
		{
			name:  "album with three 'by' occurrences",
			input: "by by by by Artist Name",
			expected: &types.Command{
				Type:     types.CommandAlbumFans,
				Album:    "by by by",
				Artist:   "Artist Name",
				RawInput: "by by by by Artist Name",
			},
			wantErr: false,
		},
		{
			name:  "track fans command",
			input: "!t Paranoid Android by Radiohead",
			expected: &types.Command{
				Type:     types.CommandTrackFans,
				Track:    "Paranoid Android",
				Artist:   "Radiohead",
				RawInput: "!t Paranoid Android by Radiohead",
			},
			wantErr: false,
		},
		{
			name:  "track with 'by' in name",
			input: "!t Led by the Heart by Nikka Costa",
			expected: &types.Command{
				Type:     types.CommandTrackFans,
				Track:    "Led by the Heart",
				Artist:   "Nikka Costa",
				RawInput: "!t Led by the Heart by Nikka Costa",
			},
			wantErr: false,
		},
		{
			name:  "recent tracks with limit",
			input: "!rp user1 10",
			expected: &types.Command{
				Type:     types.CommandRecentTracks,
				User:     "user1",
				Limit:    10,
				RawInput: "!rp user1 10",
			},
			wantErr: false,
		},
		{
			name:    "invalid user",
			input:   "!track invaliduser 7d",
			wantErr: true,
		},
		{
			name:    "invalid period",
			input:   "!track user1 invalid",
			wantErr: true,
		},
		{
			name:    "excessive limit",
			input:   "!rp user1 25",
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result.Type != tt.expected.Type {
				t.Errorf("Expected type %v, got %v", tt.expected.Type, result.Type)
			}

			if result.User != tt.expected.User {
				t.Errorf("Expected user %s, got %s", tt.expected.User, result.User)
			}

			if result.Period != tt.expected.Period {
				t.Errorf("Expected period %s, got %s", tt.expected.Period, result.Period)
			}

			if result.Artist != tt.expected.Artist {
				t.Errorf("Expected artist %s, got %s", tt.expected.Artist, result.Artist)
			}

			if result.Album != tt.expected.Album {
				t.Errorf("Expected album %s, got %s", tt.expected.Album, result.Album)
			}

			if result.Track != tt.expected.Track {
				t.Errorf("Expected track %s, got %s", tt.expected.Track, result.Track)
			}

			if result.Limit != tt.expected.Limit {
				t.Errorf("Expected limit %d, got %d", tt.expected.Limit, result.Limit)
			}
		})
	}
}

func TestParser_cleanMessage(t *testing.T) {
	parser := NewParser([]string{"user1"})

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "remove slack mention",
			input:    "<@U1234567890> !help",
			expected: "!help",
		},
		{
			name:     "normalize whitespace",
			input:    "  !track   user1   7d  ",
			expected: "!track user1 7d",
		},
		{
			name:     "combined cleaning",
			input:    "  <@U1234567890>   !chart    user1   ",
			expected: "!chart user1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.cleanMessage(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestIsValidPeriod(t *testing.T) {
	parser := NewParser([]string{"user1"})

	validPeriods := []string{"24h", "7d", "1w", "1m", "30d", "3m", "90d", "6m", "180d", "1y", "365d", "overall"}
	invalidPeriods := []string{"invalid", "2w", "5m", ""}

	for _, period := range validPeriods {
		t.Run("valid_"+period, func(t *testing.T) {
			if !parser.isValidPeriod(period) {
				t.Errorf("Expected %s to be valid", period)
			}
		})
	}

	for _, period := range invalidPeriods {
		t.Run("invalid_"+period, func(t *testing.T) {
			if parser.isValidPeriod(period) {
				t.Errorf("Expected %s to be invalid", period)
			}
		})
	}
}

func TestParser_splitOnLastBy(t *testing.T) {
	parser := NewParser([]string{"user1"})

	tests := []struct {
		name           string
		input          string
		expectedBefore string
		expectedAfter  string
		expectedFound  bool
	}{
		{
			name:           "simple case",
			input:          "OK Computer by Radiohead",
			expectedBefore: "OK Computer",
			expectedAfter:  "Radiohead",
			expectedFound:  true,
		},
		{
			name:           "multiple by occurrences",
			input:          "Songs by Tony Sly by No Use for a Name",
			expectedBefore: "Songs by Tony Sly",
			expectedAfter:  "No Use for a Name",
			expectedFound:  true,
		},
		{
			name:           "capitalized By",
			input:          "Led by the Heart By Nikka Costa",
			expectedBefore: "Led by the Heart",
			expectedAfter:  "Nikka Costa",
			expectedFound:  true,
		},
		{
			name:           "three by occurrences",
			input:          "by by by by Artist Name",
			expectedBefore: "by by by",
			expectedAfter:  "Artist Name",
			expectedFound:  true,
		},
		{
			name:           "no by separator",
			input:          "Just Artist Name",
			expectedBefore: "",
			expectedAfter:  "",
			expectedFound:  false,
		},
		{
			name:           "empty after by",
			input:          "Album Name by ",
			expectedBefore: "Album Name",
			expectedAfter:  "",
			expectedFound:  false,
		},
		{
			name:           "empty before by",
			input:          " by Artist Name",
			expectedBefore: "",
			expectedAfter:  "Artist Name",
			expectedFound:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before, after, found := parser.splitOnLastBy(tt.input)

			if found != tt.expectedFound {
				t.Errorf("Expected found %v, got %v", tt.expectedFound, found)
			}

			if before != tt.expectedBefore {
				t.Errorf("Expected before %q, got %q", tt.expectedBefore, before)
			}

			if after != tt.expectedAfter {
				t.Errorf("Expected after %q, got %q", tt.expectedAfter, after)
			}
		})
	}
}
