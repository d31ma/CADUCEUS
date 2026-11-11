package search

import "encoding/json"

type Window struct {
	Content    []json.RawMessage `json:"content"`
	NextCursor string            `json:"nextCursor"`
}
