package dkim

import (
	"regexp"
	"strings"
)

type relaxed struct{}

func (relaxed) whitespace() *regexp.Regexp {
	return regexp.MustCompile(`[ \t]+`)
}

func (relaxed) lineEnd() string {
	return "\r\n"
}

func (canon relaxed) header(name string, value string) string {
	unfolded := strings.ReplaceAll(value, canon.lineEnd(), "")
	compressed := canon.whitespace().ReplaceAllString(unfolded, " ")
	return strings.ToLower(strings.TrimSpace(name)) + ":" + strings.TrimSpace(compressed)
}

func (canon relaxed) body(body []byte) string {
	return canon.finish(strings.TrimRight(canon.lines(string(body)), canon.lineEnd()))
}

func (canon relaxed) lines(body string) string {
	lines := strings.Split(body, canon.lineEnd())
	for index, line := range lines {
		lines[index] = canon.line(line)
	}
	return strings.Join(lines, canon.lineEnd())
}

func (canon relaxed) line(line string) string {
	return strings.TrimRight(canon.whitespace().ReplaceAllString(line, " "), " ")
}

func (canon relaxed) finish(text string) string {
	if text == "" {
		return ""
	}
	return text + canon.lineEnd()
}
