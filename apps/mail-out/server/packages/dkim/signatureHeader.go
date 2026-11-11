package dkim

import (
	"encoding/base64"
	"fmt"
	"strings"
)

type signatureHeader struct {
	algorithm string
	domain    string
	selector  string
	timestamp int64
	headers   []string
	bodyHash  []byte
}

func (signatureHeader) name() string {
	return "DKIM-Signature"
}

func (signatureHeader) width() int {
	return 72
}

func (signatureHeader) fold() string {
	return "\r\n\t"
}

func (header signatureHeader) unsigned() string {
	return fmt.Sprintf("%s: v=1; a=%s; c=relaxed/relaxed; d=%s; s=%s;%st=%d; h=%s;%sbh=%s;%sb=",
		header.name(), header.algorithm, strings.ToLower(header.domain), strings.ToLower(header.selector), header.fold(),
		header.timestamp, strings.Join(header.headers, ":"), header.fold(),
		base64.StdEncoding.EncodeToString(header.bodyHash), header.fold())
}

func (header signatureHeader) canonical() string {
	_, value, _ := strings.Cut(header.unsigned(), ":")
	return relaxed{}.header(header.name(), value)
}

func (header signatureHeader) signed(signature []byte) string {
	return header.unsigned() + header.folded(base64.StdEncoding.EncodeToString(signature)) + relaxed{}.lineEnd()
}

func (header signatureHeader) folded(text string) string {
	parts := []string{}
	for start := 0; start < len(text); start += header.width() {
		parts = append(parts, text[start:min(start+header.width(), len(text))])
	}
	return strings.Join(parts, header.fold())
}
