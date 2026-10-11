import (
	"regexp"
	"strings"
)

// LinkDestinationUtils finds links whose visible text names another host or
// whose host imitates another domain, as email-content-protection.js did.
type LinkDestinationUtils struct{}

func (LinkDestinationUtils) SafeExternalTarget(value string) string {
	return LinkDestinationUtils{}.safeTarget(UrlUtils{}.ParseWeb(strings.TrimSpace(value)))
}

func (LinkDestinationUtils) Analyze(href string, label string) LinkDestination {
	address := UrlUtils{}.ParseWeb(strings.TrimSpace(href))
	destination := LinkDestination{Target: LinkDestinationUtils{}.safeTarget(address), DestinationHost: LinkDestinationUtils{}.displayHost(address.Host), DisplayedHost: LinkDestinationUtils{}.visibleHost(strings.TrimSpace(label))}
	return LinkDestinationUtils{}.verdict(destination, LinkDestinationUtils{}.isInternationalized(destination.DestinationHost, LinkDestinationUtils{}.rawHost(href)))
}

func (LinkDestinationUtils) pattern(name string) *regexp.Regexp {
	sources := map[string]string{
		"authority":  `(?i)^https?://([^/?#]+)`,
		"confusable": `[^\x{0000}-\x{007f}]|[\x{0370}-\x{03ff}\x{0400}-\x{052f}]`,
		"schemed":    `(?i)^[a-z][a-z0-9+.-]*://`,
		"user":       `^.*@`,
		"port":       `:\d+$`,
	}
	return regexp.MustCompile(sources[name])
}

func (LinkDestinationUtils) safeTarget(address WebAddress) string {
	if !address.Valid || address.HasCredentials {
		return ""
	}
	return address.Href
}

func (LinkDestinationUtils) dot() string { return "." }

func (LinkDestinationUtils) displayHost(host string) string {
	trimmed := strings.ToLower(strings.TrimSpace(host))
	return strings.TrimSuffix(strings.TrimPrefix(trimmed, "www."), LinkDestinationUtils{}.dot())
}

func (LinkDestinationUtils) visibleHost(label string) string {
	if label == "" || strings.ContainsAny(label, " \t\n\r\f\v ") {
		return ""
	}
	return LinkDestinationUtils{}.addressHost(UrlUtils{}.ParseWeb(LinkDestinationUtils{}.candidate(label)))
}

func (LinkDestinationUtils) candidate(label string) string {
	if (LinkDestinationUtils{}).pattern("schemed").MatchString(label) {
		return label
	}
	return "https://" + label
}

func (LinkDestinationUtils) addressHost(address WebAddress) string {
	if !address.Valid {
		return ""
	}
	return LinkDestinationUtils{}.displayHost(address.Host)
}

func (LinkDestinationUtils) rawHost(href string) string {
	found := LinkDestinationUtils{}.pattern("authority").FindStringSubmatch(href)
	authority := strings.Join(found[min(1, len(found)):], "")
	return LinkDestinationUtils{}.pattern("port").ReplaceAllString(LinkDestinationUtils{}.pattern("user").ReplaceAllString(authority, ""), "")
}

func (LinkDestinationUtils) isInternationalized(destination string, rawHost string) bool {
	dot := LinkDestinationUtils{}.dot()
	return strings.Contains(dot+destination, dot+"xn--") || LinkDestinationUtils{}.pattern("confusable").MatchString(rawHost)
}

func (LinkDestinationUtils) verdict(destination LinkDestination, internationalized bool) LinkDestination {
	switch {
	case destination.Target == "":
		return LinkDestination{}
	case LinkDestinationUtils{}.mismatched(destination):
		destination.Suspicious = true
		destination.Reason = "The displayed host " + destination.DisplayedHost + " does not match " + destination.DestinationHost + LinkDestinationUtils{}.dot()
	case internationalized:
		destination.Suspicious = true
		destination.Reason = "The destination " + destination.DestinationHost + " uses internationalized characters that may imitate another domain."
	}
	return destination
}

func (LinkDestinationUtils) mismatched(destination LinkDestination) bool {
	return destination.DisplayedHost != "" && destination.DisplayedHost != destination.DestinationHost
}
