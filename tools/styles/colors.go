package main

import (
	"regexp"
	"strings"
)

const propertiesLayer = "@layer properties {"

func colorValuePattern() *regexp.Regexp {
	return regexp.MustCompile(`(?i)#[0-9a-f]{3,8}\b|\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch|color|color-mix)\(`)
}

func colorKeywords() map[string]bool {
	return map[string]bool{
		"inherit":      true,
		"initial":      true,
		"unset":        true,
		"revert":       true,
		"revert-layer": true,
		"currentcolor": true,
		"transparent":  true,
		"none":         true,
	}
}

func hardCodedColors(rules string) []string {
	utilities, _, _ := strings.Cut(rules, propertiesLayer)
	pattern := colorValuePattern()
	keywords := colorKeywords()
	found := []string{}
	for _, line := range strings.Split(utilities, "\n") {
		if declaresColor(strings.TrimSpace(line), pattern, keywords) {
			found = append(found, strings.TrimSpace(line))
		}
	}
	return found
}

func declaresColor(line string, pattern *regexp.Regexp, keywords map[string]bool) bool {
	property, value, isDeclaration := declaration(line)
	if !isDeclaration {
		return false
	}
	if pattern.MatchString(value) {
		return true
	}
	return colorProperty(property) && !allowedColorValue(value, keywords)
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
