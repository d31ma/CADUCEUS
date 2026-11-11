package compose

type bodyBuilder struct {
	boundaries BoundarySource
	encoder    encoder
	writer     documentWriter
}

func (builder bodyBuilder) contentTransferEncoding() string {
	return "Content-Transfer-Encoding: "
}

func (builder bodyBuilder) build(message Message) part {
	content := builder.content(message)
	return builder.withAttachments(content, message.Attachments)
}

func (builder bodyBuilder) content(message Message) part {
	if message.HtmlBody == "" {
		return builder.text("text/plain", message.TextBody)
	}
	return builder.multipart("alternative", []part{
		builder.text("text/plain", message.TextBody),
		builder.text("text/html", message.HtmlBody),
	})
}

func (builder bodyBuilder) withAttachments(content part, attachments []Attachment) part {
	if len(attachments) == 0 {
		return content
	}
	return builder.multipart("mixed", append([]part{content}, builder.attachmentParts(attachments)...))
}

func (builder bodyBuilder) text(mediaType string, text string) part {
	return part{
		headers: []string{
			"Content-Type: " + mediaType + "; charset=utf-8",
			builder.contentTransferEncoding() + "quoted-printable",
		},
		body: builder.encoder.quotedPrintable(text),
	}
}

func (builder bodyBuilder) multipart(subtype string, parts []part) part {
	boundary := builder.boundaries.Next()
	return part{
		headers: []string{"Content-Type: multipart/" + subtype + "; boundary=\"" + boundary + "\""},
		body:    builder.writer.multipartBody(boundary, parts),
	}
}

func (builder bodyBuilder) attachmentParts(attachments []Attachment) []part {
	parts := make([]part, 0, len(attachments))
	for _, attachment := range attachments {
		parts = append(parts, builder.attachment(attachment))
	}
	return parts
}

func (builder bodyBuilder) attachment(attachment Attachment) part {
	return part{
		headers: []string{
			"Content-Type: " + builder.contentType(attachment.ContentType),
			"Content-Disposition: " + builder.encoder.fileName(attachment.FileName),
			builder.contentTransferEncoding() + "base64",
		},
		body: builder.encoder.base64Lines(attachment.Content),
	}
}

func (bodyBuilder) contentType(contentType string) string {
	if contentType == "" {
		return "application/octet-stream"
	}
	return contentType
}
