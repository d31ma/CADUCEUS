package mailin

import (
	"encoding/json"
	"errors"
)

type ContentRecord struct {
	InboundId   string `json:"inboundId"`
	Text        string `json:"text"`
	BodyFormat  string `json:"bodyFormat"`
	Truncated   bool   `json:"truncated"`
	Blocks      string `json:"blocks"`
	Attachments string `json:"attachments"`
}

func (ContentRecord) From(inboundId string, content InboundContent) ContentRecord {
	blocks, _ := json.Marshal(content.Blocks)
	attachments, _ := json.Marshal(content.Attachments)
	return ContentRecord{InboundId: inboundId, Text: content.Text, BodyFormat: content.BodyFormat, Truncated: content.Truncated, Blocks: string(blocks), Attachments: string(attachments)}
}

func (record ContentRecord) Content() (InboundContent, error) {
	content := InboundContent{Text: record.Text, BodyFormat: record.BodyFormat, Truncated: record.Truncated}
	blocksErr := json.Unmarshal([]byte(record.Blocks), &content.Blocks)
	attachmentsErr := json.Unmarshal([]byte(record.Attachments), &content.Attachments)
	return content, record.unreadable(errors.Join(blocksErr, attachmentsErr))
}

func (ContentRecord) unreadable(failure error) error {
	if failure == nil {
		return nil
	}
	return Unavailable{Operation: "stored content"}
}
