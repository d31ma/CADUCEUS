package compose

import "strings"

type documentWriter struct{}

func (writer documentWriter) render(headers []string, content part) string {
	lineEnd := limits{}.LineEnd()
	allHeaders := append(append([]string{}, headers...), writer.foldAll(content.headers)...)
	return strings.Join(allHeaders, lineEnd) + lineEnd + lineEnd + content.body
}

func (documentWriter) foldAll(headers []string) []string {
	folded := make([]string, 0, len(headers))
	for _, header := range headers {
		folded = append(folded, folder{}.fold(header))
	}
	return folded
}

func (writer documentWriter) multipartBody(boundary string, parts []part) string {
	lineEnd := limits{}.LineEnd()
	var builder strings.Builder
	for _, child := range parts {
		builder.WriteString("--" + boundary + lineEnd + writer.render(nil, child) + lineEnd)
	}
	builder.WriteString("--" + boundary + "--" + lineEnd)
	return builder.String()
}

func (writer documentWriter) document(headers []string, content part) []byte {
	return []byte(writer.endWithLineEnd(writer.render(headers, content)))
}

func (documentWriter) endWithLineEnd(text string) string {
	lineEnd := limits{}.LineEnd()
	if strings.HasSuffix(text, lineEnd) {
		return text
	}
	return text + lineEnd
}
