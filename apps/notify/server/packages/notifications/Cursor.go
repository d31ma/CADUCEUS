package notifications

import (
	"encoding/base64"
	"strings"
)

type Cursor struct{}

func (cursor Cursor) Encode(createdAt string, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(createdAt + cursor.separator() + id))
}

func (cursor Cursor) Decode(text string) (string, string, error) {
	raw, err := cursor.decodeText(text)
	if err != nil {
		return "", "", err
	}
	return cursor.split(raw)
}

func (Cursor) separator() string {
	return "\n"
}

func (Cursor) notReadable() InvalidPage {
	return InvalidPage{Reason: "cursor is not readable"}
}

func (cursor Cursor) decodeText(text string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(text)
	if err != nil {
		return "", cursor.notReadable()
	}
	return string(raw), nil
}

func (cursor Cursor) split(raw string) (string, string, error) {
	createdAt, id, found := strings.Cut(raw, cursor.separator())
	if !found || createdAt == "" {
		return "", "", cursor.notReadable()
	}
	return createdAt, id, nil
}
