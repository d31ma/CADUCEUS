import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

// TrackedHtml prepares the per-recipient HTML that SMTP delivery sends for
// tracked mail and lists the links a Sent copy can record clicks on. The
// stored Sent copy stays clean: only the delivered body carries tracking
// capabilities. TrackingId and Recipient name the capability's message and
// recipient; OpenTracking and LinkTracking choose what is tracked.
type TrackedHtml struct {
	TrackingId   string
	Recipient    string
	OpenTracking bool
	LinkTracking bool
}

// Prepared answers the delivery body with every trackable link sent through
// /track/click and, for open tracking, a pixel from /track/open appended.
// Unlike htmlparser2, which re-serialized the whole document, the rest of
// the markup is kept byte for byte.
func (tracked TrackedHtml) Prepared(source string) (string, error) {
	if source == "" || len(source) > tracked.largestBody() {
		return source, nil
	}
	base, err := tracked.BaseUrl("")
	if err != nil {
		return "", err
	}
	tokens := TrackingTokens{}
	now := EpochTime{}.Now()
	var built strings.Builder
	cursor, index := 0, 0
	if tracked.LinkTracking {
		for _, anchor := range (HtmlScanner{}).Anchors(source) {
			target := tokens.NormalizedUrl(anchor.Href)
			if index >= tokens.mostLinks() || anchor.Quoted || target == "" || tracked.endpoint(target, base) {
				continue
			}
			token, err := tokens.Created(TrackingCapability{Type: tokens.click(), TrackingId: tracked.TrackingId, Recipient: tracked.Recipient, Url: target, LinkIndex: index}, now)
			if err != nil {
				return "", err
			}
			built.WriteString(source[cursor:anchor.HrefStart])
			built.WriteString(`href="` + base + "/track/click?t=" + url.QueryEscape(token) + `"`)
			cursor = anchor.HrefEnd
			index++
		}
	}
	built.WriteString(source[cursor:])
	if tracked.OpenTracking {
		token, err := tokens.Created(TrackingCapability{Type: tokens.open(), TrackingId: tracked.TrackingId, Recipient: tracked.Recipient}, now)
		if err != nil {
			return "", err
		}
		built.WriteString(`<img alt="" aria-hidden="true" width="1" height="1" style="display:none" src="` + base + "/track/open?t=" + url.QueryEscape(token) + `">`)
	}
	return built.String(), nil
}

// TrackableLinks answers the original links that can take part in click
// analytics, as {index, url, title}.
func (tracked TrackedHtml) TrackableLinks(source string) []any {
	links := []any{}
	if source == "" || len(source) > tracked.largestBody() {
		return links
	}
	tokens := TrackingTokens{}
	for _, anchor := range (HtmlScanner{}).Anchors(source) {
		target := tokens.NormalizedUrl(anchor.Href)
		if anchor.Quoted || target == "" {
			continue
		}
		if len(links) >= tokens.mostLinks() {
			break
		}
		title := TextValue{}.Prefix(Security{}.Trim(anchor.Text), tracked.longestTitle())
		if title == "" {
			title = target
		}
		links = append(links, map[string]any{"index": len(links), "url": target, "title": title})
	}
	return links
}

// BaseUrl answers the public origin tracking links point at: configured,
// else CADUCEUS_PUBLIC_URL, else SESAME_ISSUER, else (outside production)
// http://127.0.0.1:<PORT or 8080>.
func (tracked TrackedHtml) BaseUrl(configured string) (string, error) {
	settings := Settings{}
	candidate := configured
	for _, name := range []string{"CADUCEUS_PUBLIC_URL", "SESAME_ISSUER"} {
		if candidate == "" {
			candidate = settings.Value(name)
		}
	}
	production := settings.Value("NODE_ENV") == "production"
	if candidate == "" && !production {
		port := settings.Value("PORT")
		if port == "" {
			port = "8080"
		}
		candidate = "http://127.0.0.1:" + port
	}
	parsed, err := url.Parse(candidate)
	if err != nil || parsed.Host == "" {
		return "", errors.New("CADUCEUS_PUBLIC_URL must be an absolute HTTP(S) URL")
	}
	if (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("CADUCEUS_PUBLIC_URL must be a credential-free HTTP(S) origin")
	}
	host := strings.ToLower(parsed.Hostname())
	loopback := host == "localhost" || host == "127.0.0.1" || host == "::1"
	if production && parsed.Scheme != "https" && !loopback {
		return "", errors.New("CADUCEUS_PUBLIC_URL must use HTTPS in production")
	}
	return tracked.origin(parsed), nil
}

func (TrackedHtml) origin(parsed *url.URL) string {
	host := strings.ToLower(parsed.Host)
	if (parsed.Scheme == "http" && parsed.Port() == "80") || (parsed.Scheme == "https" && parsed.Port() == "443") {
		host = strings.ToLower(parsed.Hostname())
	}
	return parsed.Scheme + "://" + host
}

func (tracked TrackedHtml) endpoint(value string, base string) bool {
	parsed, err := url.Parse(value)
	return err == nil && tracked.origin(parsed) == base && regexp.MustCompile(`^/track/(?:open|click)$`).MatchString(parsed.Path)
}

func (TrackedHtml) largestBody() int { return 2 * 1024 * 1024 }

func (TrackedHtml) longestTitle() int { return 200 }
