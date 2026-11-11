package mailin

import (
	"testing"

	"mesail.dev/packages/richtext"
)

func TestARecordKeepsTheContent(t *testing.T) {
	content := InboundContent{Text: "Hi", BodyFormat: "text", Blocks: richtext.Converter{}.FromText("Hi").Blocks, Attachments: []InboundAttachment{{FileName: "a.pdf", BlobKey: "example.com/attachment-1"}}}
	again, err := ContentRecord{}.From("in_D1", content).Content()
	check(t, err == nil && again.Text == "Hi" && again.Blocks[0].Spans[0].Text == "Hi", "wrong blocks")
	check(t, again.Attachments[0].BlobKey == "example.com/attachment-1", "wrong attachments")
}
