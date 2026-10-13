import "time"

// GitHubProfileEntry is one remembered GitHub lookup.
type GitHubProfileEntry struct {
	ExpiresAt time.Time
	Value     map[string]any
}
