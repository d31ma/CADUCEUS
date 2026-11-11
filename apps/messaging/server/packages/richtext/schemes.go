package richtext

import (
	"slices"
	"strings"
)

type schemes struct{}

func (schemes) https() string {
	return "https"
}

func (schemes) http() string {
	return "http"
}

func (rules schemes) linkSchemes() []string {
	return []string{rules.https(), rules.http(), "mailto"}
}

func (rules schemes) imageSchemes() []string {
	return []string{rules.https(), rules.http()}
}

func (rules schemes) link(address string) string {
	return rules.keep(address, rules.linkSchemes())
}

func (rules schemes) image(address string) string {
	return rules.keep(address, rules.imageSchemes())
}

func (rules schemes) keep(address string, allowed []string) string {
	trimmed := strings.TrimSpace(address)
	if !slices.Contains(allowed, rules.schemeOf(trimmed)) {
		return ""
	}
	return trimmed
}

func (schemes) schemeOf(address string) string {
	return strings.ToLower(address[:max(strings.IndexByte(address, ':'), 0)])
}
