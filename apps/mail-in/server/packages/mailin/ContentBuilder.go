package mailin

import (
	"bytes"
	"cmp"
	"errors"
	"io"
	"strings"

	"mesail.dev/packages/blobs"
	"mesail.dev/packages/mime"
	"mesail.dev/packages/richtext"
)

type ContentBuilder struct {
	Blobs blobs.Store
}

func (builder ContentBuilder) Build(stored InboundRaw) (InboundContent, error) {
	raw, err := builder.raw(stored.RawBlobKey)
	if err != nil {
		return InboundContent{}, err
	}
	message, err := mime.Parser{}.Parse(raw)
	return builder.fromMessage(stored, message, err)
}

func (builder ContentBuilder) Unreadable() InboundContent {
	return InboundContent{BodyFormat: BodyFormats{}.Text(), Blocks: []richtext.Block{}, Truncated: true, Attachments: []InboundAttachment{}}
}

func (builder ContentBuilder) raw(path string) ([]byte, error) {
	reader, err := builder.Blobs.Read(BlobKeys{}.FromPath(path))
	if err != nil {
		return nil, Unavailable{Operation: "raw message"}
	}
	defer reader.Close()
	raw, err := io.ReadAll(io.LimitReader(reader, int64(mime.Parser{}.MaxSize())+1))
	return raw, builder.failure("raw message", err)
}

func (builder ContentBuilder) fromMessage(stored InboundRaw, message mime.Message, parseErr error) (InboundContent, error) {
	if parseErr != nil {
		return builder.Unreadable(), nil
	}
	attachments, err := builder.saved(stored, message.Attachments)
	return builder.converted(message, attachments), err
}

func (builder ContentBuilder) saved(stored InboundRaw, parts []mime.Attachment) ([]InboundAttachment, error) {
	attachments := []InboundAttachment{}
	failures := []error{}
	for index, part := range parts {
		key := BlobKeys{}.Attachment(stored.DomainId, stored.RawBlobKey, index)
		failures = append(failures, builder.Blobs.Write(key, bytes.NewReader(part.Content), int64(len(part.Content))))
		attachments = append(attachments, InboundAttachment{FileName: part.FileName, MediaType: part.MediaType, Size: int64(len(part.Content)), BlobKey: key.Path(), ContentId: part.ContentId})
	}
	return attachments, builder.failure("attachment blob", errors.Join(failures...))
}

func (builder ContentBuilder) converted(message mime.Message, attachments []InboundAttachment) InboundContent {
	message.Text = builder.withoutFinalBreak(message.Text)
	result, format := builder.blocks(message, builder.inlineImages(attachments))
	return InboundContent{Text: cmp.Or(message.Text, result.Text), BodyFormat: format, Blocks: result.Blocks, Truncated: result.Truncated, Attachments: attachments}
}

func (ContentBuilder) blocks(message mime.Message, inlineImages map[string]string) (richtext.Result, string) {
	if message.Html != "" {
		return richtext.Converter{}.FromHtml(message.Html, inlineImages), BodyFormats{}.Email()
	}
	return richtext.Converter{}.FromText(message.Text), BodyFormats{}.Text()
}

func (builder ContentBuilder) withoutFinalBreak(text string) string {
	if strings.HasSuffix(text, builder.windowsBreak()) {
		return strings.TrimSuffix(text, builder.windowsBreak())
	}
	return strings.TrimSuffix(text, builder.unixBreak())
}

func (ContentBuilder) windowsBreak() string {
	return "\r\n"
}

func (ContentBuilder) unixBreak() string {
	return "\n"
}

func (ContentBuilder) inlineImages(attachments []InboundAttachment) map[string]string {
	images := map[string]string{}
	for _, attachment := range attachments {
		images[attachment.ContentId] = attachment.BlobKey
	}
	delete(images, "")
	return images
}

func (ContentBuilder) failure(operation string, err error) error {
	if err == nil {
		return nil
	}
	return Unavailable{Operation: operation}
}
