package main

import (
	"regexp"
	"strings"
)

const (
	classAttribute        = "class"
	boundClassAttribute   = ":class"
	templateLiteralMarker = "`"
)

type attribute struct {
	name  string
	value string
}

func commentPattern() *regexp.Regexp {
	return regexp.MustCompile(`(?s)<!--.*?-->`)
}

func startTagPattern() *regexp.Regexp {
	return regexp.MustCompile(`<[A-Za-z][\w:.-]*(?:"[^"]*"|'[^']*'|[^'">])*>`)
}

func attributePattern() *regexp.Regexp {
	return regexp.MustCompile(`\s([^\s"'<>/=]+)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'=<>` + "`" + `]+)))?`)
}

func stringLiteralPattern() *regexp.Regexp {
	return regexp.MustCompile(`'((?:[^'\\]|\\.)*)'|"((?:[^"\\]|\\.)*)"|` + "`((?:[^`\\\\]|\\\\.)*)`")
}

func templateExpressionPattern() *regexp.Regexp {
	return regexp.MustCompile(`\$\{[^}]*\}`)
}

func classSource(template []byte) string {
	lines := []string{}
	for _, item := range templateAttributes(string(template)) {
		lines = append(lines, classValues(item)...)
	}
	return strings.Join(lines, "\n") + "\n"
}

func templateAttributes(template string) []attribute {
	markup := commentPattern().ReplaceAllString(template, "")
	pattern := attributePattern()
	found := []attribute{}
	for _, tag := range startTagPattern().FindAllString(markup, -1) {
		found = append(found, tagAttributes(tag, pattern)...)
	}
	return found
}

func tagAttributes(tag string, pattern *regexp.Regexp) []attribute {
	found := []attribute{}
	for _, match := range pattern.FindAllStringSubmatch(tag, -1) {
		found = append(found, attribute{name: strings.ToLower(match[1]), value: match[2] + match[3] + match[4]})
	}
	return found
}

func classValues(item attribute) []string {
	switch item.name {
	case classAttribute:
		return nonBlank([]string{item.value})
	case boundClassAttribute:
		return literalParts(item.value)
	}
	return nil
}

func literalParts(expression string) []string {
	parts := []string{}
	for _, match := range stringLiteralPattern().FindAllStringSubmatch(expression, -1) {
		parts = append(parts, literalText(match))
	}
	return nonBlank(parts)
}

func literalText(match []string) string {
	if strings.HasPrefix(match[0], templateLiteralMarker) {
		return templateExpressionPattern().ReplaceAllString(match[3], " ")
	}
	return match[1] + match[2]
}

func nonBlank(values []string) []string {
	kept := []string{}
	for _, value := range values {
		if trimmed := strings.Join(strings.Fields(value), " "); trimmed != "" {
			kept = append(kept, trimmed)
		}
	}
	return kept
}
