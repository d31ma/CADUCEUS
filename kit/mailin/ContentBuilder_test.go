package mailin

import (
	"strings"
	"testing"
)

func TestTheBuilderMarksAnUnreadableMessageAsTruncated(t *testing.T) {
	memory, store := newMemory(), newBlobs()
	stored := storedMessage(t, memory, store, "no headers here")
	content, err := ContentBuilder{Blobs: store}.Build(stored)
	check(t, err == nil && content.Truncated && content.BodyFormat == "text", "want an empty truncated body")
	check(t, len(content.Blocks) == 0 && len(content.Attachments) == 0, "want empty lists")
}

func TestTheBuilderReportsAFailedRead(t *testing.T) {
	memory, store := newMemory(), newBlobs()
	stored := storedMessage(t, memory, store, textMessage())
	store.failOn = "read"
	_, err := ContentBuilder{Blobs: store}.Build(stored)
	check(t, isUnavailable(err, "raw message"), "want raw message")
}

func TestTheBuilderReportsAFailedAttachmentWrite(t *testing.T) {
	memory, store := newMemory(), newBlobs()
	stored := storedMessage(t, memory, store, htmlMessage())
	store.failOn = "write"
	_, err := ContentBuilder{Blobs: store}.Build(stored)
	check(t, isUnavailable(err, "attachment blob"), "want attachment blob")
}

func TestTheBuilderKeepsAnHtmlImageWithoutAContentIdRemote(t *testing.T) {
	memory, store := newMemory(), newBlobs()
	raw := strings.Replace(htmlMessage(), "cid:logo@outside.test", "cid:", 1)
	stored := storedMessage(t, memory, store, raw)
	content, err := ContentBuilder{Blobs: store}.Build(stored)
	check(t, err == nil && imageBlock(content).Image.AttachmentId == "", "an empty content id named a blob")
}
