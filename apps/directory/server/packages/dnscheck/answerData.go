package dnscheck

import "strings"

type answerData struct {
	hosts hostName
}

func (data answerData) mxHost(raw string) string {
	_, host, _ := strings.Cut(strings.TrimSpace(raw), data.space())
	return data.hosts.normalize(host)
}

func (data answerData) text(raw string) string {
	inner := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(raw), data.quote()), data.quote())
	joined := strings.ReplaceAll(inner, data.pieceBreak(), "")
	return data.unescaper().Replace(joined)
}

func (data answerData) unescaper() *strings.Replacer {
	return strings.NewReplacer(data.escapedQuote(), data.quote(), data.escapedBackslash(), data.backslash())
}

func (answerData) space() string {
	return " "
}

func (answerData) quote() string {
	return `"`
}

func (answerData) pieceBreak() string {
	return `" "`
}

func (answerData) escapedQuote() string {
	return `\"`
}

func (answerData) escapedBackslash() string {
	return `\\`
}

func (answerData) backslash() string {
	return `\`
}
