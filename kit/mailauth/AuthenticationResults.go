package mailauth

import (
	"regexp"
	"strings"
)

type authenticationResults struct{}

func (header authenticationResults) field(results Results, authServId string) string {
	groups := append([][]string{header.spf(results)}, header.dkim(results.Dkim)...)
	groups = append(groups, header.dmarc(results))
	words := header.join([]string{header.name(), header.clean(authServId)}, groups)
	return header.fold(words) + header.newline()
}

func (authenticationResults) name() string {
	return "Authentication-Results:"
}

func (header authenticationResults) spf(results Results) []string {
	return append([]string{"spf=" + header.clean(results.Spf)}, header.property("smtp.mailfrom", results.SpfDomain)...)
}

func (header authenticationResults) dkim(dkim []DkimResult) [][]string {
	groups := header.signatures(dkim)
	return append(groups, header.noSignature(len(groups))...)
}

func (header authenticationResults) signatures(dkim []DkimResult) [][]string {
	var groups [][]string
	for _, result := range dkim {
		groups = append(groups, header.signature(result))
	}
	return groups
}

func (header authenticationResults) signature(result DkimResult) []string {
	words := []string{"dkim=" + header.clean(result.Result)}
	words = append(words, header.property("header.d", result.Domain)...)
	return append(words, header.property("header.s", result.Selector)...)
}

func (authenticationResults) noSignature(count int) [][]string {
	if count > 0 {
		return nil
	}
	return [][]string{{"dkim=" + Outcomes{}.None()}}
}

func (header authenticationResults) dmarc(results Results) []string {
	return append([]string{"dmarc=" + header.clean(results.Dmarc)}, header.property("header.from", results.FromDomain)...)
}

func (header authenticationResults) property(name string, value string) []string {
	cleaned := header.clean(value)
	if cleaned == "" {
		return nil
	}
	return []string{name + "=" + cleaned}
}

func (authenticationResults) clean(value string) string {
	return regexp.MustCompile(`[^A-Za-z0-9._@+-]`).ReplaceAllString(value, "")
}

func (authenticationResults) join(words []string, groups [][]string) []string {
	for _, group := range groups {
		words[len(words)-1] += ";"
		words = append(words, group...)
	}
	return words
}

func (header authenticationResults) fold(words []string) string {
	lines := []string{words[0]}
	for _, word := range words[1:] {
		lines = header.place(lines, word)
	}
	return strings.Join(lines, header.newline())
}

func (header authenticationResults) place(lines []string, word string) []string {
	last := len(lines) - 1
	if len(lines[last])+len(" ")+len(word) > header.width() {
		return append(lines, "\t"+word)
	}
	lines[last] += " " + word
	return lines
}

func (authenticationResults) newline() string {
	return "\r\n"
}

func (authenticationResults) width() int {
	return 78
}
