import (
	"maps"

	"mesail.dev/packages/core"
)

// @Service
type WireService struct {
}

func (service *WireService) ConversationPage(list core.ConversationList) ConversationPage {
	return ConversationPage{Items: service.conversations(list.Items), NextCursor: list.NextCursor}
}

func (service *WireService) MessagePage(list core.MessageList) MessagePage {
	return MessagePage{Items: service.messages(list.Items), NextCursor: list.NextCursor}
}

func (service *WireService) SendResult(outcome core.SendOutcome) SendResult {
	return SendResult{Message: service.Message(outcome.Message), Deliveries: service.deliveries(outcome.Deliveries)}
}

func (service *WireService) Message(view core.MessageView) Message {
	message := view.Message
	return Message{
		MessageId:          core.MessageIds{}.Wire(message.Month, message.Id),
		ConversationId:     message.ConversationId,
		ThreadRootId:       message.ThreadRootId,
		AlsoToConversation: message.AlsoToConversation,
		Author:             service.author(message),
		Body:               message.Body,
		BodyFormat:         message.BodyFormat,
		Blocks:             service.blocks(view.Blocks),
		Truncated:          message.Truncated,
		Attachments:        service.attachments(message.AttachmentIds),
		Email:              service.email(message),
		Reactions:          service.counts(view.Reactions),
		MyReactions:        service.texts(view.MyReactions),
		ReplyCount:         message.ReplyCount,
		LastReplyAt:        message.LastReplyAt,
		EditedAt:           message.EditedAt,
		DeletedAt:          message.DeletedAt,
		CreatedAt:          message.CreatedAt,
		Deliveries:         service.deliveries(view.Deliveries),
	}
}

func (service *WireService) SendRequest(request SendRequest) core.SendRequest {
	return core.SendRequest{
		ClientMessageId:    request.ClientMessageId,
		ConversationId:     request.ConversationId,
		ThreadRootId:       request.ThreadRootId,
		AlsoToConversation: request.AlsoToConversation,
		Body:               request.Body,
		AttachmentIds:      service.texts(request.AttachmentIds),
		Recipients:         service.choices(request.Recipients),
		Email:              service.emailOptions(request.Email),
	}
}

func (service *WireService) MessageSource(source core.MessageSource) MessageSource {
	return MessageSource{MessageId: source.MessageId, DomainId: source.DomainId, Body: source.Body, AttachmentIds: service.texts(source.AttachmentIds)}
}

func (service *WireService) Download(download core.Download) Download {
	return Download{Url: download.Url, ExpiresAt: download.ExpiresAt}
}

func (service *WireService) conversations(views []core.ConversationView) []ConversationSummary {
	summaries := []ConversationSummary{}
	for _, view := range views {
		summaries = append(summaries, service.conversation(view))
	}
	return summaries
}

func (service *WireService) conversation(view core.ConversationView) ConversationSummary {
	return ConversationSummary{
		ConversationId: view.ConversationId,
		Kind:           view.Conversation.Kind,
		Title:          view.Title,
		HasExternal:    view.Conversation.HasExternal,
		UnreadCount:    view.UnreadCount,
		MentionCount:   view.MentionCount,
		LastMessageAt:  view.Conversation.LastMessageAt,
	}
}

func (service *WireService) messages(views []core.MessageView) []Message {
	messages := []Message{}
	for _, view := range views {
		messages = append(messages, service.Message(view))
	}
	return messages
}

func (service *WireService) author(message core.Message) Author {
	return Author{Kind: message.AuthorKind, MemberId: message.AuthorMemberId, Address: message.AuthorAddress, DisplayName: message.AuthorDisplayName}
}

func (service *WireService) blocks(blocks []core.Block) []Block {
	converted := []Block{}
	for _, block := range blocks {
		converted = append(converted, service.block(block))
	}
	return converted
}

func (service *WireService) block(block core.Block) Block {
	return Block{
		Kind:    block.Kind,
		Level:   block.Level,
		Ordered: block.Ordered,
		Spans:   service.spans(block.Spans),
		Image:   service.image(block.Image),
		Cells:   service.texts(block.Cells),
	}
}

func (service *WireService) spans(spans []core.Span) []Span {
	converted := []Span{}
	for _, span := range spans {
		converted = append(converted, Span{Text: span.Text, Bold: span.Bold, Italic: span.Italic, Code: span.Code, Strike: span.Strike, Href: span.Href})
	}
	return converted
}

func (service *WireService) image(image *core.ImageRef) *ImageRef {
	if image == nil {
		return nil
	}
	return &ImageRef{Alt: image.Alt, AttachmentId: image.AttachmentId, RemoteUrl: image.RemoteUrl}
}

func (service *WireService) attachments(attachmentIds []string) []AttachmentRef {
	references := []AttachmentRef{}
	for _, attachmentId := range attachmentIds {
		references = append(references, AttachmentRef{AttachmentId: attachmentId})
	}
	return references
}

func (service *WireService) email(message core.Message) *EmailSummary {
	if !message.IsEmail {
		return nil
	}
	return service.emailSummary(message.Email)
}

func (service *WireService) emailSummary(email core.EmailMeta) *EmailSummary {
	return &EmailSummary{
		Subject:     email.Subject,
		From:        email.From,
		To:          service.texts(email.To),
		Cc:          service.texts(email.Cc),
		Suspicious:  email.Suspicious,
		AuthResults: AuthResults{Spf: email.Spf, Dkim: email.Dkim, Dmarc: email.Dmarc},
	}
}

func (service *WireService) deliveries(records []core.DeliveryRecord) []DeliveryStatus {
	statuses := []DeliveryStatus{}
	for _, record := range records {
		statuses = append(statuses, DeliveryStatus{Recipient: record.Recipient, Channel: record.Channel, Status: record.Status, Detail: record.Detail})
	}
	return statuses
}

func (service *WireService) choices(choices []RecipientChoice) []core.RecipientChoice {
	converted := []core.RecipientChoice{}
	for _, choice := range choices {
		converted = append(converted, core.RecipientChoice{Address: choice.Address, Role: choice.Role, Delivery: choice.Delivery, ExpectedKind: choice.ExpectedKind})
	}
	return converted
}

func (service *WireService) emailOptions(options *EmailOptions) core.EmailOptions {
	if options == nil {
		return core.EmailOptions{}
	}
	return core.EmailOptions{Subject: options.Subject, FromAddress: options.FromAddress, SendToExternal: options.SendToExternal}
}

func (service *WireService) counts(reactions map[string]int) map[string]int {
	counts := map[string]int{}
	maps.Copy(counts, reactions)
	return counts
}

func (service *WireService) texts(values []string) []string {
	return append([]string{}, values...)
}
