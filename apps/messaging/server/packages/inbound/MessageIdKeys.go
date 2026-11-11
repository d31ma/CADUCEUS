package inbound

import "strings"

type MessageIdKeys struct{}

func (keys MessageIdKeys) Of(messageIdHeader string) string {
	return Digests{}.Hex(strings.ToLower(strings.Trim(strings.TrimSpace(messageIdHeader), keys.brackets())))
}

func (MessageIdKeys) brackets() string {
	return "<>"
}
