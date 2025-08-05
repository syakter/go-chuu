package buildinfo

import (
	"runtime"
	"strings"
	"testing"
)

func TestGet(t *testing.T) {
	info := Get()

	// Test that we get back an Info struct with expected fields
	if info.GoVersion == "" {
		t.Error("GoVersion should not be empty")
	}

	if info.GoOS == "" {
		t.Error("GoOS should not be empty")
	}

	if info.GoArch == "" {
		t.Error("GoArch should not be empty")
	}

	// Verify runtime values match what Go reports
	if info.GoVersion != runtime.Version() {
		t.Errorf("Expected GoVersion %s, got %s", runtime.Version(), info.GoVersion)
	}

	if info.GoOS != runtime.GOOS {
		t.Errorf("Expected GoOS %s, got %s", runtime.GOOS, info.GoOS)
	}

	if info.GoArch != runtime.GOARCH {
		t.Errorf("Expected GoArch %s, got %s", runtime.GOARCH, info.GoArch)
	}
}

func TestString(t *testing.T) {
	tests := []struct {
		name     string
		info     Info
		expected string
	}{
		{
			name: "with build time",
			info: Info{
				Version:   "v1.0.0",
				GitCommit: "abc123def456",
				BuildTime: "2023-01-01T12:00:00Z",
			},
			expected: "v1.0.0 (abc123de, 2023-01-01 12:00:00)",
		},
		{
			name: "without build time",
			info: Info{
				Version:   "v1.0.0",
				GitCommit: "abc123def456",
				BuildTime: "unknown",
			},
			expected: "v1.0.0 (abc123de)",
		},
		{
			name: "short commit",
			info: Info{
				Version:   "v1.0.0",
				GitCommit: "abc123",
				BuildTime: "unknown",
			},
			expected: "v1.0.0 (abc123)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.info.String()
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestDefaults(t *testing.T) {
	// Test that default values are sensible
	if Version == "" {
		t.Error("Version should have a default value")
	}

	if GitCommit == "" {
		t.Error("GitCommit should have a default value")
	}

	if GitBranch == "" {
		t.Error("GitBranch should have a default value")
	}

	if BuildTime == "" {
		t.Error("BuildTime should have a default value")
	}
}

func TestBuildTimeFormatting(t *testing.T) {
	// Test that valid RFC3339 timestamps are formatted correctly
	testTime := "2023-12-25T10:30:45Z"
	info := Info{
		Version:   "test",
		GitCommit: "abcdef123456",
		BuildTime: testTime,
	}

	result := info.String()
	if !strings.Contains(result, "2023-12-25 10:30:45") {
		t.Errorf("Expected formatted time in result, got: %s", result)
	}
}

func TestInvalidBuildTimeFormatting(t *testing.T) {
	// Test that invalid timestamps fall back to simple format
	info := Info{
		Version:   "test",
		GitCommit: "abcdef123456",
		BuildTime: "invalid-time",
	}

	result := info.String()
	expected := "test (abcdef12)"
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}
