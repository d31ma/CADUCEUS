package mailin

import "encoding/json"

type Page struct {
	Content    json.RawMessage
	NextCursor string
}
