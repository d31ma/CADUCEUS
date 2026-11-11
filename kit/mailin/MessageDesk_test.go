package mailin

import (
	"strings"
	"testing"

	"mesail.dev/packages/richtext"
)

func TestOriginalSignsADownloadOfTheRawMessage(t *testing.T) {
	memory, store := newMemory(), newBlobs()
	inboundId := acceptedMessage(t, memory, store, "example.com/inbound-1", textMessage())
	download, err := testMessageDesk(memory, store, "example.com").Original("Bearer service", inboundId)
	check(t, err == nil && strings.HasPrefix(download.Url, "https://blobs.test/example.com/inbound-1"), "wrong link")
	check(t, download.ExpiresAt == "2026-11-10T22:05:00Z", "want five minutes")
	check(t, store.signed[0].FileName == "message.eml" && store.signed[0].MediaType == "message/rfc822", "wrong terms")
}

func TestOriginalHidesAMessageOfAnotherDomain(t *testing.T) {
	memory, store := newMemory(), newBlobs()
	inboundId := acceptedMessage(t, memory, store, "example.com/inbound-1", textMessage())
	_, err := testMessageDesk(memory, store, "example.org").Original("Bearer service", inboundId)
	check(t, isMissing(err) && len(store.signed) == 0, "want missing")
}

func TestOriginalRefusesARefusedToken(t *testing.T) {
	memory, store := newMemory(), newBlobs()
	_, err := testMessageDesk(memory, store, "example.com").Original("", "in_D1")
	check(t, err == Unauthenticated{} && err.Error() != "", "want unauthenticated")
}

func TestOriginalReportsASigningFailure(t *testing.T) {
	memory, store := newMemory(), newBlobs()
	inboundId := acceptedMessage(t, memory, store, "example.com/inbound-1", textMessage())
	store.failOn = "sign"
	_, err := testMessageDesk(memory, store, "example.com").Original("Bearer service", inboundId)
	check(t, isUnavailable(err, "download link"), "want download link")
}

func TestContentConvertsAnHtmlEmailWithAnInlineImageAndAPdf(t *testing.T) {
	memory, store := newMemory(), newBlobs()
	inboundId := acceptedMessage(t, memory, store, "example.com/inbound-1", htmlMessage())
	content, err := testMessageDesk(memory, store, "example.com").Content("Bearer service", inboundId)
	check(t, err == nil && content.BodyFormat == "email", "want an email body")
	check(t, len(content.Attachments) == 2, "want two attachments")
	image, pdf := content.Attachments[0], content.Attachments[1]
	check(t, image.ContentId == "logo@outside.test" && image.MediaType == "image/png", "wrong image")
	check(t, pdf.FileName == "plan.pdf" && pdf.MediaType == "application/pdf" && pdf.ContentId == "", "wrong pdf")
	check(t, string(store.objects[pdf.BlobKey]) == "%PDF-1.4\n" && pdf.Size == 9, "the pdf was not stored")
	check(t, strings.HasPrefix(image.BlobKey, "example.com/attachment-"), "wrong key")
	check(t, imageBlock(content).Image.AttachmentId == image.BlobKey, "want the image to name its blob key")
	check(t, !strings.Contains(content.Text, "alert"), "the script was kept")
}

func TestContentConvertsAPlainTextEmail(t *testing.T) {
	memory, store := newMemory(), newBlobs()
	inboundId := acceptedMessage(t, memory, store, "example.com/inbound-1", textMessage())
	content, err := testMessageDesk(memory, store, "example.com").Content("Bearer service", inboundId)
	check(t, err == nil && content.BodyFormat == "text", "want a text body")
	check(t, strings.HasPrefix(content.Text, "Lunch at noon?"), "wrong text")
	check(t, content.Blocks[0].Spans[0].Text == "Lunch at noon?", "wrong block")
	check(t, len(content.Attachments) == 0 && !content.Truncated, "wrong content")
}

func TestASecondContentCallReadsTheStoredResult(t *testing.T) {
	memory, store := newMemory(), newBlobs()
	inboundId := acceptedMessage(t, memory, store, "example.com/inbound-1", htmlMessage())
	desk := testMessageDesk(memory, store, "example.com")
	first, _ := desk.Content("Bearer service", inboundId)
	delete(store.objects, "example.com/inbound-1")
	second, err := desk.Content("Bearer service", inboundId)
	check(t, err == nil, "the second call read the raw message")
	check(t, second.BodyFormat == first.BodyFormat && len(second.Blocks) == len(first.Blocks), "want the stored blocks")
	check(t, second.Attachments[1].BlobKey == first.Attachments[1].BlobKey, "want the stored attachments")
}

func TestContentHidesAMessageOfAnotherDomain(t *testing.T) {
	memory, store := newMemory(), newBlobs()
	inboundId := acceptedMessage(t, memory, store, "example.com/inbound-1", textMessage())
	_, err := testMessageDesk(memory, store, "example.org").Content("Bearer service", inboundId)
	check(t, isMissing(err), "want missing")
}

func imageBlock(content InboundContent) richtext.Block {
	for _, block := range content.Blocks {
		if block.Image != nil {
			return block
		}
	}
	return richtext.Block{Image: &richtext.ImageRef{}}
}
