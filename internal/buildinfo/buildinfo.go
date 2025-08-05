package buildinfo

import (
	"runtime"
	"time"
)

// Build information variables set at compile time via ldflags
var (
	// Version is the application version
	Version = "dev"
	// GitCommit is the git commit hash
	GitCommit = "unknown"
	// GitBranch is the git branch
	GitBranch = "unknown"
	// BuildTime is when the binary was built
	BuildTime = "unknown"
	// GoVersion is the Go version used to build
	GoVersion = runtime.Version()
)

// Info represents build information
type Info struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
	GitBranch string `json:"git_branch"`
	BuildTime string `json:"build_time"`
	GoVersion string `json:"go_version"`
	GoOS      string `json:"go_os"`
	GoArch    string `json:"go_arch"`
}

// Get returns the current build information
func Get() Info {
	return Info{
		Version:   Version,
		GitCommit: GitCommit,
		GitBranch: GitBranch,
		BuildTime: BuildTime,
		GoVersion: GoVersion,
		GoOS:      runtime.GOOS,
		GoArch:    runtime.GOARCH,
	}
}

// String returns a human-readable representation of build info
func (i Info) String() string {
	if i.BuildTime != "unknown" {
		if parsed, err := time.Parse(time.RFC3339, i.BuildTime); err == nil {
			return i.Version + " (" + i.GitCommit[:min(8, len(i.GitCommit))] + ", " + parsed.Format("2006-01-02 15:04:05") + ")"
		}
	}
	return i.Version + " (" + i.GitCommit[:min(8, len(i.GitCommit))] + ")"
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
