package textindex

import (
	"cmp"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

type Cursor struct {
	CreatedAt  string `json:"c"`
	DocumentId string `json:"d"`
}

func (cursor Cursor) Encode() string {
	text, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(text)
}

func (cursor Cursor) Decode(encoded string) (Cursor, error) {
	return cursor.parsed(base64.RawURLEncoding.DecodeString(encoded))
}

func (Cursor) parsed(text []byte, failure error) (Cursor, error) {
	var decoded Cursor
	if errors.Join(failure, json.Unmarshal(text, &decoded)) != nil {
		return Cursor{}, BadCursor{}
	}
	return decoded, nil
}

func (cursor Cursor) Admits(createdAt string, documentId string) bool {
	return cursor.admits(Cursor{CreatedAt: createdAt, DocumentId: documentId})
}

func (cursor Cursor) admits(item Cursor) bool {
	return cursor.isStart() || item.compare(cursor) < 0
}

func (cursor Cursor) isStart() bool {
	return cursor == Cursor{}
}

func (cursor Cursor) compare(other Cursor) int {
	return cmp.Or(strings.Compare(cursor.CreatedAt, other.CreatedAt), strings.Compare(cursor.DocumentId, other.DocumentId))
}
