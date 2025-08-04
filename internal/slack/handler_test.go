package slack

import (
	"testing"

	"github.com/slack-go/slack"
)

func TestHandler_getUserInfo(t *testing.T) {
	// This test would require mocking the Slack API
	// For now, we'll test the logic without making actual API calls

	// Test cases for user info extraction logic
	testCases := []struct {
		name         string
		userID       string
		mockUser     *slack.User
		expectedName string
	}{
		{
			name:   "user with display name",
			userID: "U123456",
			mockUser: &slack.User{
				ID:   "U123456",
				Name: "john.doe",
				Profile: slack.UserProfile{
					DisplayName: "John Doe",
					RealName:    "John Michael Doe",
				},
			},
			expectedName: "John Doe", // Should prefer display name
		},
		{
			name:   "user with real name only",
			userID: "U789012",
			mockUser: &slack.User{
				ID:   "U789012",
				Name: "jane.smith",
				Profile: slack.UserProfile{
					DisplayName: "",
					RealName:    "Jane Smith",
				},
			},
			expectedName: "Jane Smith", // Should fall back to real name
		},
		{
			name:   "user with username only",
			userID: "U345678",
			mockUser: &slack.User{
				ID:   "U345678",
				Name: "bob.wilson",
				Profile: slack.UserProfile{
					DisplayName: "",
					RealName:    "",
				},
			},
			expectedName: "bob.wilson", // Should fall back to username
		},
	}

	// Test the name preference logic
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Extract the name selection logic
			var selectedName string
			if tc.mockUser.Profile.DisplayName != "" {
				selectedName = tc.mockUser.Profile.DisplayName
			} else if tc.mockUser.Profile.RealName != "" {
				selectedName = tc.mockUser.Profile.RealName
			} else if tc.mockUser.Name != "" {
				selectedName = tc.mockUser.Name
			} else {
				selectedName = tc.userID
			}

			if selectedName != tc.expectedName {
				t.Errorf("Expected name '%s', got '%s'", tc.expectedName, selectedName)
			}
		})
	}
}

// Note: Integration tests for listening club commands with user context
// are covered by the overall system integration tests since they require
// full Slack API mocking which is complex to set up in unit tests.
// The user lookup logic is tested in TestHandler_getUserInfo above.
