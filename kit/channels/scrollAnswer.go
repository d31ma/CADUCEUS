package channels

import "encoding/json"

type scrollAnswer struct {
	Content    []json.RawMessage `json:"content"`
	NextCursor string            `json:"nextCursor"`
}
