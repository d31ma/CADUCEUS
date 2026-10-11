import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

// UrlUtils reads absolute web and mailto addresses the way the browser URL
// parser does for the cases the reader checks: scheme, credentials, host,
// port and the normalized href.
type UrlUtils struct{}

func (UrlUtils) ParseWeb(raw string) WebAddress {
	return UrlUtils{}.webAddress(UrlUtils{}.split(UrlUtils{}.cleaned(raw)))
}

func (UrlUtils) ParseMailto(raw string) MailtoAddress {
	return UrlUtils{}.mailtoAddress(UrlUtils{}.split(UrlUtils{}.cleaned(raw)))
}

func (UrlUtils) space() byte { return ' ' }

func (UrlUtils) cleaned(raw string) string {
	trimmed := strings.TrimFunc(raw, func(character rune) bool { return character <= rune(UrlUtils{}.space()) })
	return strings.NewReplacer("\t", "", "\n", "", "\r", "").Replace(trimmed)
}

func (UrlUtils) split(text string) UrlParts {
	found := regexp.MustCompile(`(?s)^([A-Za-z][A-Za-z0-9+.\-]*):(.*)$`).FindStringSubmatch(text)
	if found == nil {
		return UrlParts{}
	}
	return UrlUtils{}.schemeParts(strings.ToLower(found[1]), found[2])
}

func (UrlUtils) schemeParts(scheme string, rest string) UrlParts {
	if (UrlUtils{}).isWebScheme(scheme) {
		return UrlUtils{}.authorityParts(scheme, strings.TrimLeft(rest, "/\\"))
	}
	return UrlParts{Scheme: scheme, Rest: rest}
}

func (UrlUtils) isWebScheme(scheme string) bool {
	return scheme == "http" || scheme == "https"
}

func (UrlUtils) authorityParts(scheme string, rest string) UrlParts {
	end := strings.IndexAny(rest, "/\\?#")
	if end < 0 {
		return UrlParts{Scheme: scheme, Authority: rest}
	}
	return UrlParts{Scheme: scheme, Authority: rest[:end], Rest: rest[end:]}
}

func (UrlUtils) userSeparator() string { return "@" }

func (UrlUtils) webAddress(parts UrlParts) WebAddress {
	if !(UrlUtils{}).isWebScheme(parts.Scheme) {
		return WebAddress{}
	}
	at := strings.LastIndex(parts.Authority, UrlUtils{}.userSeparator())
	return UrlUtils{}.hostAddress(parts, parts.Authority[:max(at, 0)], parts.Authority[at+1:])
}

func (UrlUtils) hostAddress(parts UrlParts, credentials string, hostAndPort string) WebAddress {
	host, port := UrlUtils{}.splitPort(hostAndPort)
	normalized := UrlUtils{}.normalizedHost(host)
	address := WebAddress{Scheme: parts.Scheme, HasCredentials: UrlUtils{}.hasCredentials(credentials), Host: normalized, Port: UrlUtils{}.port(parts.Scheme, port)}
	address.Valid = UrlUtils{}.validHost(normalized, port)
	address.Href = parts.Scheme + "://" + UrlUtils{}.userInfo(credentials) + normalized + UrlUtils{}.portSuffix(address.Port) + UrlUtils{}.normalizedRest(parts.Rest)
	return address
}

func (UrlUtils) colon() string { return ":" }

func (UrlUtils) hasCredentials(credentials string) bool {
	return credentials != "" && credentials != UrlUtils{}.colon()
}

func (UrlUtils) validHost(host string, port string) bool {
	return host != "" && regexp.MustCompile(`^[0-9]*$`).MatchString(port)
}

func (UrlUtils) bracketClose() string { return "]" }

func (UrlUtils) splitPort(hostAndPort string) (string, string) {
	colon := strings.LastIndex(hostAndPort, UrlUtils{}.colon())
	if colon < 0 || strings.HasSuffix(hostAndPort, (UrlUtils{}).bracketClose()) {
		return hostAndPort, ""
	}
	return hostAndPort[:colon], hostAndPort[colon+1:]
}

func (UrlUtils) normalizedHost(host string) string {
	decoded, failure := url.PathUnescape(host)
	forbidden := regexp.MustCompile(`[\x{0000}-\x{0020}#%/:<>?@\[\\\]^|\x{007f}]`).MatchString(UrlUtils{}.withoutBrackets(decoded))
	if failure != nil || decoded == "" || forbidden {
		return ""
	}
	return PunycodeUtils{}.Host(strings.ToLower(decoded))
}

func (UrlUtils) withoutBrackets(host string) string {
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, (UrlUtils{}).bracketClose()) {
		return strings.ReplaceAll(host[1:len(host)-1], UrlUtils{}.colon(), "")
	}
	return host
}

func (UrlUtils) zero() string { return "0" }

func (UrlUtils) port(scheme string, port string) string {
	trimmed := strings.TrimLeft(port, UrlUtils{}.zero())
	defaults := map[string]string{"http": "80", "https": "443"}
	if trimmed == defaults[scheme] || port == "" {
		return ""
	}
	return TextUtils{}.FirstOf(trimmed, UrlUtils{}.zero())
}

func (UrlUtils) portSuffix(port string) string {
	return TextUtils{}.Choose(port == "", "", UrlUtils{}.colon()+port)
}

func (UrlUtils) userInfo(credentials string) string {
	return TextUtils{}.Choose(credentials == "", "", credentials+UrlUtils{}.userSeparator())
}

func (UrlUtils) hash() string { return "#" }

func (UrlUtils) normalizedRest(rest string) string {
	beforeFragment, fragment, hasFragment := strings.Cut(rest, UrlUtils{}.hash())
	encodedFragment := UrlUtils{}.encoded(fragment, " \"<>`")
	return UrlUtils{}.withQuery(beforeFragment) + TextUtils{}.Choose(hasFragment, UrlUtils{}.hash()+encodedFragment, "")
}

func (UrlUtils) question() string { return "?" }

func (UrlUtils) withQuery(rest string) string {
	path, query, hasQuery := strings.Cut(rest, UrlUtils{}.question())
	encodedQuery := UrlUtils{}.encoded(query, " \"#<>'")
	return UrlUtils{}.normalizedPath(path) + TextUtils{}.Choose(hasQuery, UrlUtils{}.question()+encodedQuery, "")
}

func (UrlUtils) slash() string { return "/" }

func (UrlUtils) normalizedPath(path string) string {
	segments := strings.Split(strings.ReplaceAll(path, "\\", UrlUtils{}.slash()), UrlUtils{}.slash())
	kept := []string{}
	for _, segment := range segments[min(1, len(segments)):] {
		kept = UrlUtils{}.withSegment(kept, segment)
	}
	return UrlUtils{}.slash() + UrlUtils{}.encoded(strings.Join(kept, UrlUtils{}.slash()), " \"<>`{}#?") + UrlUtils{}.trailingSlash(segments[len(segments)-1], len(segments))
}

func (UrlUtils) current() string { return "." }

func (UrlUtils) parent() string { return ".." }

func (UrlUtils) withSegment(kept []string, segment string) []string {
	dots := strings.ReplaceAll(strings.ToLower(segment), "%2e", UrlUtils{}.current())
	switch dots {
	case UrlUtils{}.current():
		return kept
	case UrlUtils{}.parent():
		return kept[:max(len(kept)-1, 0)]
	}
	return append(kept, segment)
}

func (UrlUtils) trailingSlash(last string, count int) string {
	dotted := slices.Contains([]string{UrlUtils{}.current(), UrlUtils{}.parent()}, last)
	if count > 1 && dotted {
		return UrlUtils{}.slash()
	}
	return ""
}

func (UrlUtils) encoded(text string, reserved string) string {
	var builder strings.Builder
	for _, value := range []byte(text) {
		builder.WriteString(UrlUtils{}.encodedByte(value, reserved))
	}
	return builder.String()
}

func (UrlUtils) encodedByte(value byte, reserved string) string {
	space := UrlUtils{}.space()
	if value < space || value > '~' || strings.IndexByte(reserved, value) >= 0 {
		return fmt.Sprintf("%%%02X", value)
	}
	return string(rune(value))
}

func (UrlUtils) mailtoAddress(parts UrlParts) MailtoAddress {
	if parts.Scheme != "mailto" {
		return MailtoAddress{}
	}
	return UrlUtils{}.mailtoFields(parts.Rest)
}

func (UrlUtils) mailtoFields(rest string) MailtoAddress {
	beforeFragment, _, _ := strings.Cut(rest, UrlUtils{}.hash())
	path, query, _ := strings.Cut(beforeFragment, UrlUtils{}.question())
	values, _ := url.ParseQuery(query)
	mailbox, failure := url.PathUnescape(path)
	address := MailtoAddress{Valid: failure == nil, Mailbox: mailbox, Subject: values.Get("subject"), Body: values.Get("body")}
	address.HasAuthority = strings.HasPrefix(path, UrlUtils{}.slash()+UrlUtils{}.slash())
	return address
}
