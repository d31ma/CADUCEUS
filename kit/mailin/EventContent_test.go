package mailin

import (
	"testing"

	"mesail.dev/packages/richtext"
)

func TestEventContentKeepsTheBlocksAndAttachments(t *testing.T) {
	inbound := InboundContent{Text: "Hi", BodyFormat: "email", Blocks: []richtext.Block{{Kind: "paragraph"}}, Truncated: true, Attachments: []InboundAttachment{{FileName: "a.pdf", MediaType: "application/pdf", Size: 3, BlobKey: "example.com/a", ContentId: "a"}}}
	converted, err := EventContent{}.From(inbound)
	check(t, err == nil && converted.Text == "Hi" && converted.BodyFormat == "email" && converted.Truncated, "wrong fields")
	check(t, string(converted.Blocks)[:20] == `[{"kind":"paragraph"`, "wrong blocks: "+string(converted.Blocks))
	check(t, converted.Attachments[0].BlobKey == "example.com/a" && converted.Attachments[0].ContentId == "a", "wrong attachment")
}
