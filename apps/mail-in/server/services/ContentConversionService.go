import (
	"mesail.dev/packages/mailin"
	"mesail.dev/packages/richtext"
)

// @Service
type ContentConversionService struct {
}

func (service *ContentConversionService) Content(content mailin.InboundContent) InboundContent {
	return InboundContent{
		Text:        content.Text,
		BodyFormat:  content.BodyFormat,
		Blocks:      service.blocks(content.Blocks),
		Truncated:   content.Truncated,
		Attachments: service.attachments(content.Attachments),
	}
}

func (service *ContentConversionService) blocks(blocks []richtext.Block) []Block {
	converted := []Block{}
	for _, block := range blocks {
		converted = append(converted, service.block(block))
	}
	return converted
}

func (service *ContentConversionService) block(block richtext.Block) Block {
	return Block{
		Kind:    block.Kind,
		Level:   block.Level,
		Ordered: block.Ordered,
		Spans:   service.spans(block.Spans),
		Image:   service.image(block.Image),
		Cells:   append([]string{}, block.Cells...),
	}
}

func (service *ContentConversionService) spans(spans []richtext.Span) []Span {
	converted := []Span{}
	for _, span := range spans {
		converted = append(converted, Span{Text: span.Text, Bold: span.Bold, Italic: span.Italic, Code: span.Code, Strike: span.Strike, Href: span.Href})
	}
	return converted
}

func (service *ContentConversionService) image(image *richtext.ImageRef) *ImageRef {
	if image == nil {
		return nil
	}
	return &ImageRef{Alt: image.Alt, AttachmentId: image.AttachmentId, RemoteUrl: image.RemoteUrl}
}

func (service *ContentConversionService) attachments(attachments []mailin.InboundAttachment) []InboundAttachment {
	converted := []InboundAttachment{}
	for _, attachment := range attachments {
		converted = append(converted, InboundAttachment{FileName: attachment.FileName, MediaType: attachment.MediaType, Size: attachment.Size, BlobKey: attachment.BlobKey, ContentId: attachment.ContentId})
	}
	return converted
}
