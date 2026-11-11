package mailauth

import (
	"regexp"
	"strings"
)

type dkimCanonical struct{}

func (dkimCanonical) simple() string  { return "simple" }
func (dkimCanonical) relaxed() string { return "relaxed" }
func (dkimCanonical) newline() string { return "\r\n" }

func (canonical dkimCanonical) methods() []string {
	return []string{canonical.simple(), canonical.relaxed()}
}

func (canonical dkimCanonical) header(raw string, method string) string {
	return canonical.headerMethods()[method](raw)
}

func (canonical dkimCanonical) body(body string, method string) string {
	return canonical.bodyMethods()[method](body)
}

func (canonical dkimCanonical) headerMethods() map[string]func(string) string {
	return map[string]func(string) string{canonical.simple(): canonical.simpleHeader, canonical.relaxed(): canonical.relaxedHeader}
}

func (canonical dkimCanonical) bodyMethods() map[string]func(string) string {
	return map[string]func(string) string{canonical.simple(): canonical.simpleBody, canonical.relaxed(): canonical.relaxedBody}
}

func (dkimCanonical) simpleHeader(raw string) string {
	return raw
}

func (canonical dkimCanonical) relaxedHeader(raw string) string {
	name, value, _ := strings.Cut(raw, ":")
	unfolded := strings.ReplaceAll(value, canonical.newline(), "")
	compact := canonical.spaces().ReplaceAllString(unfolded, " ")
	return strings.ToLower(strings.TrimSpace(name)) + ":" + strings.Trim(compact, " \t") + canonical.newline()
}

func (canonical dkimCanonical) simpleBody(body string) string {
	return canonical.trailingLines().ReplaceAllString(body, "") + canonical.newline()
}

func (canonical dkimCanonical) relaxedBody(body string) string {
	compact := canonical.spaces().ReplaceAllString(body, " ")
	trimmed := strings.TrimSuffix(strings.ReplaceAll(compact, " "+canonical.newline(), canonical.newline()), " ")
	return canonical.terminated(canonical.trailingLines().ReplaceAllString(trimmed, ""))
}

func (canonical dkimCanonical) terminated(body string) string {
	if body == "" {
		return body
	}
	return body + canonical.newline()
}

func (canonical dkimCanonical) withoutSignature(raw string) string {
	name, value, _ := strings.Cut(raw, ":")
	pattern := regexp.MustCompile(`(^|;)([ \t\r\n]*b[ \t\r\n]*=)[^;]*`)
	return name + ":" + pattern.ReplaceAllString(value, "${1}${2}")
}

func (dkimCanonical) spaces() *regexp.Regexp {
	return regexp.MustCompile(`[ \t]+`)
}

func (dkimCanonical) trailingLines() *regexp.Regexp {
	return regexp.MustCompile(`(?:\r\n)*\z`)
}
