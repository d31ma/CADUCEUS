import (
	"regexp"
	"strings"
)

// AttachmentPolicy turns untrusted MIME attachment names and Content-IDs
// into values that are safe as storage keys, MIME output, HTML and download
// names.
type AttachmentPolicy struct{}

func (AttachmentPolicy) controlOrDirectional() *regexp.Regexp {
	return regexp.MustCompile(`[\x{0000}-\x{001f}\x{007f}-\x{009f}\x{202a}-\x{202e}\x{2066}-\x{2069}]`)
}

// SanitizeAttachmentFilename answers one portable display and download name:
// the last path segment without controls, bidirectional overrides, reserved
// path characters, leading dots or trailing dots and spaces, at most 160
// UTF-16 units, or fallback when nothing is left.
func (policy AttachmentPolicy) SanitizeAttachmentFilename(value string, fallback string) string {
	untrusted := policy.controlOrDirectional().ReplaceAllString(value, "")
	segments := strings.FieldsFunc(untrusted, func(letter rune) bool { return letter == '/' || letter == '\\' })
	leaf := ""
	if len(segments) > 0 {
		leaf = segments[len(segments)-1]
	}
	text := TextValue{}
	normalized := regexp.MustCompile(`[/\\?%*:|"<>]`).ReplaceAllString(leaf, "-")
	normalized = text.Collapsed(normalized)
	normalized = strings.TrimLeft(normalized, ".")
	normalized = strings.TrimRight(normalized, ". ")
	normalized = text.Trim(normalized)
	if normalized == "" {
		normalized = fallback
	}
	return text.Prefix(normalized, policy.longestFilename())
}

func (AttachmentPolicy) longestFilename() int { return 160 }

// NormalizeAttachmentContentId answers the Content-ID a `cid:` URL names,
// without brackets or the cid: prefix, or "" when it holds white space,
// controls or brackets or is longer than 255 UTF-16 units.
func (policy AttachmentPolicy) NormalizeAttachmentContentId(value string) string {
	text := TextValue{}
	normalized := regexp.MustCompile(`(?i)^cid:`).ReplaceAllString(text.Trim(value), "")
	if strings.HasPrefix(normalized, "<") && strings.HasSuffix(normalized, ">") && len(normalized) > 1 {
		normalized = text.Trim(normalized[1 : len(normalized)-1])
	}
	if normalized == "" || text.Length(normalized) > policy.longestContentId() {
		return ""
	}
	if policy.controlOrDirectional().MatchString(normalized) || strings.ContainsAny(normalized, "<>") || strings.IndexFunc(normalized, text.Space) >= 0 {
		return ""
	}
	return normalized
}

func (AttachmentPolicy) longestContentId() int { return 255 }
