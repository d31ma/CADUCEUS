import (
	"html"
	"regexp"
	"strings"
)

// GitHubNotification finds the one "View it on GitHub" target of a
// notification sent by github.com, without trusting other links in the mail.
type GitHubNotification struct{}

func (GitHubNotification) largestHtml() int { return 8 * 1024 * 1024 }

func (GitHubNotification) fromGitHub(value any) bool {
	text, isText := value.(string)
	if !isText {
		return false
	}
	for index, address := range (MailAddresses{}).Found(text) {
		if index >= 10 {
			break
		}
		if parts := strings.Split(address, "@"); len(parts) == 2 && parts[1] == "github.com" {
			return true
		}
	}
	return false
}

func (GitHubNotification) decoded(value string) string {
	value = regexp.MustCompile(`(?i)&amp;`).ReplaceAllLiteralString(value, "&")
	value = regexp.MustCompile(`(?i)&quot;`).ReplaceAllLiteralString(value, `"`)
	value = regexp.MustCompile(`(?i)&#39;|&apos;`).ReplaceAllLiteralString(value, "'")
	value = regexp.MustCompile(`(?i)&lt;`).ReplaceAllLiteralString(value, "<")
	value = regexp.MustCompile(`(?i)&gt;`).ReplaceAllLiteralString(value, ">")
	return regexp.MustCompile(`(?i)&#(x[0-9a-f]+|\d+);`).ReplaceAllStringFunc(value, func(entity string) string {
		decoded := html.UnescapeString(strings.ToLower(entity))
		if decoded == strings.ToLower(entity) || decoded == "�" {
			return ""
		}
		return decoded
	})
}

func (GitHubNotification) safeUrl(value string) string {
	if value == "" || (TextValue{}).Length(value) > 2048 || regexp.MustCompile(`[\x00-\x1f\x7f]`).MatchString(value) {
		return ""
	}
	parsed, href, valid := WebAddress{}.Parsed(value)
	if !valid || parsed.Scheme != "https" || parsed.User != nil || parsed.Port() != "" {
		return ""
	}
	if host := parsed.Hostname(); host != "github.com" && host != "www.github.com" {
		return ""
	}
	return href
}

// Target answers the GitHub URL of a notification from github.com, or "".
func (notification GitHubNotification) Target(message map[string]any) string {
	if !notification.fromGitHub(message["headerFrom"]) && !notification.fromGitHub(message["sender"]) {
		return ""
	}
	body, _ := message["htmlBody"].(string)
	if body == "" || len(body) > notification.largestHtml() {
		return ""
	}
	view := regexp.MustCompile(`(?i)\bview(?:\s+(?:it|this|thread|issue|pull\s+request|comment))?\s+on\s+github\b`)
	href := regexp.MustCompile(`(?i)\bhref\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
	for _, match := range regexp.MustCompile(`(?is)<a\b([^>]*)>(.*?)</a\s*>`).FindAllStringSubmatch(body, -1) {
		label := TextValue{}.Trim(TextValue{}.Collapsed(notification.decoded(regexp.MustCompile(`<[^>]*>`).ReplaceAllString(match[2], " "))))
		if !view.MatchString(label) {
			continue
		}
		link := ""
		if found := href.FindStringSubmatch(match[1]); found != nil {
			link = found[1] + found[2] + found[3]
		}
		if target := notification.safeUrl(notification.decoded(link)); target != "" {
			return target
		}
	}
	return ""
}
