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

func TestTheBuilderDropsTheFinalCrlfOfAPlainTextBody(t *testing.T) {
	content := builtText(t, "Lunch at noon?\r\nSee you.\r\n")
	check(t, content.Text == "Lunch at noon?\r\nSee you.", "the final line break was kept in the text")
	check(t, lastSpanText(content) == "Lunch at noon?\nSee you.", "the final line break was kept in the last span")
}

func TestTheBuilderDropsTheFinalLfOfAPlainTextBody(t *testing.T) {
	content := builtText(t, "Lunch at noon?\n")
	check(t, content.Text == "Lunch at noon?", "the final line feed was kept")
}

func TestTheBuilderDropsOnlyOneFinalLineBreak(t *testing.T) {
	content := builtText(t, "Lunch at noon?\r\n\r\n")
	check(t, content.Text == "Lunch at noon?\r\n", "more than one line break was removed")
}

func TestTheBuilderKeepsABodyWithoutAFinalLineBreak(t *testing.T) {
	content := builtText(t, "Lunch at noon?")
	check(t, content.Text == "Lunch at noon?" && lastSpanText(content) == "Lunch at noon?", "the body changed")
}

func TestTheBuilderDropsTheFinalCrlfOfTheTextPartBesideHtml(t *testing.T) {
	memory, store := newMemory(), newBlobs()
	stored := storedMessage(t, memory, store, alternativeMessage())
	content, err := ContentBuilder{Blobs: store}.Build(stored)
	check(t, err == nil && content.BodyFormat == "email", "want an email body")
	check(t, content.Text == "Hello there", "the final line break was kept in the text")
}

func builtText(t *testing.T, body string) InboundContent {
	memory, store := newMemory(), newBlobs()
	raw := strings.Replace(textMessage(), "Lunch at noon?\r\n", body, 1)
	stored := storedMessage(t, memory, store, raw)
	content, err := ContentBuilder{Blobs: store}.Build(stored)
	check(t, err == nil && content.BodyFormat == "text", "want a text body")
	return content
}

func lastSpanText(content InboundContent) string {
	block := content.Blocks[len(content.Blocks)-1]
	return block.Spans[len(block.Spans)-1].Text
}
