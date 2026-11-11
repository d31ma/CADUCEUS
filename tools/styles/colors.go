package main

import (
	"regexp"
	"strings"
)

const propertiesLayer = "@layer properties {"

func colorValuePattern() *regexp.Regexp {
	return regexp.MustCompile(`(?i)#[0-9a-f]{3,8}\b|\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch|color|color-mix)\(`)
}

func tokenMixPattern() *regexp.Regexp {
	part := `(?:var\(--[\w-]+\)|transparent|currentcolor)(?:\s+\d+(?:\.\d+)?%)?`
	return regexp.MustCompile(`(?i)^color-mix\(in [a-z-]+,\s*` + part + `,\s*` + part + `\)$`)
}

func systemColors() []string {
	return []string{
		"accentcolor", "accentcolortext", "activetext", "buttonborder", "buttonface", "buttontext",
		"canvas", "canvastext", "field", "fieldtext", "graytext", "highlight", "highlighttext",
		"linktext", "mark", "marktext", "selecteditem", "selecteditemtext", "visitedtext",
	}
}

func colorKeywords() map[string]bool {
	keywords := map[string]bool{
		"inherit":      true,
		"initial":      true,
		"unset":        true,
		"revert":       true,
		"revert-layer": true,
		"currentcolor": true,
		"transparent":  true,
		"none":         true,
	}
	for _, name := range systemColors() {
		keywords[name] = true
	}
	return keywords
}

func hardCodedColors(rules string) []string {
	utilities, _, _ := strings.Cut(rules, propertiesLayer)
	guard := colorGuard{values: colorValuePattern(), tokenMix: tokenMixPattern(), keywords: colorKeywords()}
	found := []string{}
	for _, line := range strings.Split(utilities, "\n") {
		if guard.declaresColor(strings.TrimSpace(line)) {
			found = append(found, strings.TrimSpace(line))
		}
	}
	return found
}

type colorGuard struct {
	values   *regexp.Regexp
	tokenMix *regexp.Regexp
	keywords map[string]bool
}

func (guard colorGuard) declaresColor(line string) bool {
	property, value, isDeclaration := declaration(line)
	if !isDeclaration || guard.tokenMix.MatchString(value) {
		return false
	}
	if guard.values.MatchString(value) {
		return true
	}
	return colorProperty(property) && !allowedColorValue(value, guard.keywords)
}

func declaration(line string) (string, string, bool) {
	if !strings.HasSuffix(line, ";") || strings.HasPrefix(line, "@") {
		return "", "", false
	}
	property, value, found := strings.Cut(strings.TrimSuffix(line, ";"), ":")
	return strings.TrimSpace(property), strings.TrimSpace(value), found
}

func colorProperty(property string) bool {
	switch property {
	case "color", "background", "fill", "stroke":
		return true
	}
	return strings.HasSuffix(property, "-color")
}

func allowedColorValue(value string, keywords map[string]bool) bool {
	if strings.HasPrefix(value, "var(") {
		return true
	}
	return keywords[strings.ToLower(value)]
}
