package events

import "encoding/json"

type EmailContent struct {
	Text        string            `json:"text"`
	BodyFormat  string            `json:"bodyFormat"`
	Blocks      json.RawMessage   `json:"blocks"`
	Truncated   bool              `json:"truncated"`
	Attachments []EmailAttachment `json:"attachments"`
}
