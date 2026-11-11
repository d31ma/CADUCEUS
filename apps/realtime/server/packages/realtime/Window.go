package realtime

import "encoding/json"

type Window struct {
	Content    json.RawMessage
	NextCursor string
}
