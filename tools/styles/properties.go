package main

import (
	"regexp"
	"sort"
	"strings"
)

const foundationHeader = "/*! Tailwind CSS " + tailwindVersion + " variable registrations | MIT License | https://tailwindcss.com */"

func propertyPattern() *regexp.Regexp {
	return regexp.MustCompile(`(?m)^@property\s+(--[\w-]+)\s*\{[^}]*\}\n?`)
}

func blankLinesPattern() *regexp.Regexp {
	return regexp.MustCompile(`\n{3,}`)
}

func splitRegistrations(css string) (string, map[string]string) {
	pattern := propertyPattern()
	registrations := map[string]string{}
	for _, match := range pattern.FindAllStringSubmatch(css, -1) {
		registrations[match[1]] = strings.TrimSpace(match[0])
	}
	rules := pattern.ReplaceAllString(css, "")
	rules = blankLinesPattern().ReplaceAllString(rules, "\n\n")
	return strings.TrimSpace(rules), registrations
}

func hasRules(rules string) bool {
	return strings.Contains(rules, "{")
}

func foundationBody(registrations map[string]string) string {
	if len(registrations) == 0 {
		return ""
	}
	names := make([]string, 0, len(registrations))
	for name := range registrations {
		names = append(names, name)
	}
	sort.Strings(names)
	blocks := make([]string, 0, len(names))
	for _, name := range names {
		blocks = append(blocks, registrations[name])
	}
	return foundationHeader + "\n" + strings.Join(blocks, "\n\n")
}
