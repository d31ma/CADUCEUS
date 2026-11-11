package mailin

import "mesail.dev/packages/richtext"

type InboundContent struct {
	Text        string              `json:"text"`
	BodyFormat  string              `json:"bodyFormat"`
	Blocks      []richtext.Block    `json:"blocks"`
	Truncated   bool                `json:"truncated"`
	Attachments []InboundAttachment `json:"attachments"`
}
