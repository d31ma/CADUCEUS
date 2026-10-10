import (
	"regexp"
	"strings"
)

// CategoryPath checks a provider folder or label path: lower-case, bounded,
// and without empty, "." or ".." segments.
type CategoryPath struct{}

// Normalized answers the path in its normal form, or "" when it is unsafe.
func (path CategoryPath) Normalized(value any) string {
	text, isText := value.(string)
	if !isText {
		return ""
	}
	normalized := strings.ToLower(Security{}.Trim(text))
	if normalized == "" || len(normalized) > path.maximumLength() || !regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._/-]{0,58}[a-z0-9])?$`).MatchString(normalized) {
		return ""
	}
	if strings.Contains(normalized, "//") {
		return ""
	}
	for _, part := range strings.Split(normalized, "/") {
		if part == "." || part == ".." {
			return ""
		}
	}
	return normalized
}

func (CategoryPath) maximumLength() int { return 60 }
