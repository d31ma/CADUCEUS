package mailout

import "encoding/json"

type Window struct {
	Content    json.RawMessage `json:"content"`
	NextCursor string          `json:"nextCursor"`
}
