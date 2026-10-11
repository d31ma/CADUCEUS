import (
	"net/url"
	"strings"
)

// WebAddress reads an http or https URL the way a browser serialises it
// (WHATWG href): surrounding blanks trimmed, tabs and line breaks removed,
// scheme and host in lower case, the default port dropped and an empty path
// written as "/".
type WebAddress struct{}

// Parsed answers the URL and its href, or false when raw is not an http or
// https URL with a host.
func (WebAddress) Parsed(raw string) (*url.URL, string, bool) {
	cleaned := strings.NewReplacer("\t", "", "\n", "", "\r", "").Replace(strings.TrimFunc(raw, func(letter rune) bool { return letter <= ' ' }))
	parsed, err := url.Parse(cleaned)
	if err != nil {
		return nil, "", false
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return nil, "", false
	}
	host := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	if (parsed.Scheme == "http" && port == "80") || (parsed.Scheme == "https" && port == "443") {
		port = ""
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	parsed.Host = host
	if port != "" {
		parsed.Host = host + ":" + port
	}
	if parsed.Path == "" && parsed.Opaque == "" {
		parsed.Path = "/"
	}
	return parsed, parsed.String(), true
}

// Href answers the href of an http or https URL ("//host" means https), or
// "" for anything else.
func (address WebAddress) Href(value any) string {
	raw := TextValue{}.Trim(TextValue{}.Of(value))
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}
	_, href, valid := address.Parsed(raw)
	if !valid {
		return ""
	}
	return href
}
