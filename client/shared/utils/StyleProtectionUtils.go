import (
	"regexp"
	"slices"
	"strings"
)

// StyleProtectionUtils applies the remote-content policy to CSS and to the
// URLs that mail markup loads, as email-content-protection.js did.
type StyleProtectionUtils struct{}

func (StyleProtectionUtils) IsRemoteContentUrl(value string) bool {
	return StyleProtectionUtils{}.remote(strings.TrimSpace(value))
}

func (StyleProtectionUtils) IsKnownTrackingUrl(value string) bool {
	return StyleProtectionUtils{}.tracked(StyleProtectionUtils{}.withScheme(strings.TrimSpace(value)))
}

func (StyleProtectionUtils) NormalizeRemoteUrl(value string) string {
	return StyleProtectionUtils{}.validHref(UrlUtils{}.ParseWeb(StyleProtectionUtils{}.httpsRelative(strings.TrimSpace(value))))
}

func (StyleProtectionUtils) IsSafeEmbeddedSource(value string) bool {
	return StyleProtectionUtils{}.pattern("embedded").MatchString(value)
}

func (StyleProtectionUtils) ProtectCss(css string, options ProtectionOptions) ProtectedCss {
	result := &ProtectedCss{RemoteUrls: []string{}}
	stripped := StyleProtectionUtils{}.pattern("fonts").ReplaceAllString(StyleProtectionUtils{}.pattern("imports").ReplaceAllString(css, ""), "")
	result.Css = StyleProtectionUtils{}.rewriteUrls(stripped, options, result)
	return *result
}

func (StyleProtectionUtils) RememberUrl(urls []string, url string) []string {
	return StyleProtectionUtils{}.remembered(urls, url)
}

func (StyleProtectionUtils) pattern(name string) *regexp.Regexp {
	sources := map[string]string{
		"network":  `(?i)^https?:`,
		"imports":  `(?i)@import\s+(?:url\(\s*)?(?:['"][^'"]+['"]|[^;\s)]+)\s*\)?\s*;?`,
		"fonts":    `(?i)@font-face\s*\{[^}]*\}`,
		"function": `(?i)url\(\s*(?:"([^'")]+)"|'([^'")]+)'|([^'")]+))\s*\)`,
		"embedded": `(?i)^data:image/(?:png|jpeg|gif|webp|avif);base64,[A-Za-z0-9+/]+=*$`,
	}
	return regexp.MustCompile(sources[name])
}

func (StyleProtectionUtils) trackers() []*regexp.Regexp {
	sources := []string{
		`(?:^|\.)click\.ngpvan\.com/`, `(?:^|\.)bl-1\.com/`, `mailstat\.us/tr`, `tracking\.cirrusinsight\.com`,
		`(?:^|\.)t\.yesware\.com/`, `app\.yesware\.com/`, `mailfoogae\.appspot\.com`, `launchbit\.com/taz-pixel`,
		`list-manage\.com/track`, `cmail1\.com/t(?:[/?]|$)`, `click\.icptrack\.com/icp/`, `infusionsoft\.com/app/emailopened`,
		`via\.intercom\.io/o(?:[/?]|$)`, `mandrillapp\.com/track`, `(?:^|\.)t\.hsms06\.com/`, `app\.relateiq\.com/t\.png`,
		`api\.mixpanel\.com/track`, `web\.frontapp\.com/api`, `mailtrack\.io/trace`, `sdr\.salesloft\.com/email_trackers`,
		`(?:^|\.)t\.(?:signaux|senal|sidekickopen|sigopn)(?:[./]|$)`, `/track/open(?:[/?#]|$)`,
	}
	patterns := []*regexp.Regexp{}
	for _, source := range sources {
		patterns = append(patterns, regexp.MustCompile("(?i)"+source))
	}
	return patterns
}

func (StyleProtectionUtils) relative() string { return "//" }

func (StyleProtectionUtils) remote(url string) bool {
	return StyleProtectionUtils{}.pattern("network").MatchString(url) || strings.HasPrefix(url, StyleProtectionUtils{}.relative())
}

func (StyleProtectionUtils) tracked(normalized string) bool {
	return normalized != "" && slices.ContainsFunc(StyleProtectionUtils{}.trackers(), func(pattern *regexp.Regexp) bool { return pattern.MatchString(normalized) })
}

func (StyleProtectionUtils) withScheme(value string) string {
	return StyleProtectionUtils{}.prefixed(value, "https://"+strings.TrimPrefix(value, StyleProtectionUtils{}.relative()))
}

func (StyleProtectionUtils) httpsRelative(value string) string {
	return StyleProtectionUtils{}.prefixed(value, "https:"+value)
}

func (StyleProtectionUtils) prefixed(value string, absolute string) string {
	if strings.HasPrefix(value, (StyleProtectionUtils{}).relative()) {
		return absolute
	}
	return value
}

func (StyleProtectionUtils) validHref(address WebAddress) string {
	if !address.Valid {
		return ""
	}
	return address.Href
}

func (StyleProtectionUtils) rewriteUrls(css string, options ProtectionOptions, result *ProtectedCss) string {
	var builder strings.Builder
	cursor := 0
	for _, found := range (StyleProtectionUtils{}).pattern("function").FindAllStringSubmatchIndex(css, -1) {
		builder.WriteString(css[cursor:found[0]])
		builder.WriteString(StyleProtectionUtils{}.replacement(css[found[0]:found[1]], StyleProtectionUtils{}.urlValue(css, found), options, result))
		cursor = found[1]
	}
	builder.WriteString(css[cursor:])
	return builder.String()
}

func (StyleProtectionUtils) urlValue(css string, found []int) string {
	value := ""
	for group := 2; group+1 < len(found); group += 2 {
		value += css[max(found[group], 0):max(found[group+1], 0)]
	}
	return strings.TrimSpace(value)
}

func (StyleProtectionUtils) replacement(match string, url string, options ProtectionOptions, result *ProtectedCss) string {
	switch {
	case AttachmentContentUtils{}.IsContentReference(url):
		return StyleProtectionUtils{}.embedded(options.InlineSources[AttachmentContentUtils{}.NormalizeContentReference(url)])
	case !StyleProtectionUtils{}.IsRemoteContentUrl(url):
		return match
	}
	return StyleProtectionUtils{}.remoteReplacement(url, options, result)
}

func (StyleProtectionUtils) blockedValue() string { return "none" }

func (StyleProtectionUtils) embedded(source string) string {
	if (StyleProtectionUtils{}).IsSafeEmbeddedSource(source) {
		return `url("` + source + `")`
	}
	return StyleProtectionUtils{}.blockedValue()
}

func (StyleProtectionUtils) remoteReplacement(url string, options ProtectionOptions, result *ProtectedCss) string {
	normalized := StyleProtectionUtils{}.NormalizeRemoteUrl(url)
	result.RemoteCount++
	if (StyleProtectionUtils{}).IsKnownTrackingUrl(url) {
		result.TrackingCount++
		return StyleProtectionUtils{}.blockedValue()
	}
	result.RemoteUrls = StyleProtectionUtils{}.remembered(result.RemoteUrls, normalized)
	return StyleProtectionUtils{}.allowedSource(options.RemoteSources[normalized], options.AllowRemote, result)
}

func (StyleProtectionUtils) allowedSource(source string, allowRemote bool, result *ProtectedCss) string {
	if !allowRemote || !(StyleProtectionUtils{}).IsSafeEmbeddedSource(source) {
		result.BlockedCount++
		return StyleProtectionUtils{}.blockedValue()
	}
	return StyleProtectionUtils{}.embedded(source)
}

func (StyleProtectionUtils) remembered(urls []string, url string) []string {
	if url == "" || slices.Contains(urls, url) {
		return urls
	}
	return append(urls, url)
}
