import (
	"regexp"
	"slices"
)

// GitHubNotificationUtils revalidates server-derived GitHub targets at the
// last browser boundary, as github-notification.js did.
type GitHubNotificationUtils struct{}

func (GitHubNotificationUtils) SafeTarget(value string) string {
	address := UrlUtils{}.ParseWeb(value)
	return GitHubNotificationUtils{}.target(value, address)
}

func (GitHubNotificationUtils) controls() *regexp.Regexp {
	return regexp.MustCompile(`[\x{0000}-\x{001f}\x{007f}]`)
}

func (GitHubNotificationUtils) maximumLength() int { return 2048 }

func (GitHubNotificationUtils) target(value string, address WebAddress) string {
	tooLong := len(value) > GitHubNotificationUtils{}.maximumLength()
	controls := GitHubNotificationUtils{}.controls().MatchString(value)
	github := slices.Contains([]string{"github.com", "www.github.com"}, address.Host)
	if value == "" || tooLong || controls || !address.Valid || address.Scheme != "https" || address.HasCredentials || address.Port != "" || !github {
		return ""
	}
	return address.Href
}
