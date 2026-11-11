package inbound

import (
	"encoding/json"
	"time"

	"mesail.dev/packages/core"
)

type EmailMessages struct{}

func (messages EmailMessages) New(email ReceivedEmail, placement Placement, content core.InboundContent, now time.Time) (core.Message, error) {
	blocks, err := json.Marshal(append([]core.Block{}, content.Blocks...))
	if err != nil {
		return core.Message{}, err
	}
	sender := EmailParticipants{}.Sender(email, "")
	return core.Message{
		DomainId:           email.DomainId,
		ConversationId:     core.ConversationIds{}.Wire(placement.Conversation),
		ThreadRootId:       messages.rootId(placement),
		Month:              core.MessageIds{}.Month(now),
		Sequence:           placement.Conversation.MessageCount + 1,
		AuthorKind:         sender.Kind,
		AuthorAddress:      sender.Address,
		SendKey:            email.SendKey,
		Body:               content.Text,
		BodyFormat:         content.BodyFormat,
		BlocksJson:         string(blocks),
		Truncated:          content.Truncated,
		AttachmentIds:      []string{},
		MentionedMemberIds: []string{},
		IsEmail:            true,
		Email:              messages.meta(email),
		Deliveries:         []string{},
		CreatedAt:          Timestamps{}.Of(now),
	}, nil
}

func (EmailMessages) rootId(placement Placement) string {
	if !placement.HasRoot {
		return ""
	}
	return core.MessageIds{}.Wire(placement.Root.Month, placement.Root.Id)
}

func (EmailMessages) meta(email ReceivedEmail) core.EmailMeta {
	headers := email.Mail.Headers
	results := email.Mail.AuthResults
	return core.EmailMeta{
		MessageIdHeader: headers.MessageId,
		InReplyTo:       headers.InReplyTo,
		References:      append([]string{}, headers.References...),
		Subject:         headers.Subject,
		From:            headers.From,
		To:              append([]string{}, headers.To...),
		Cc:              append([]string{}, headers.Cc...),
		Spf:             results.Spf,
		Dkim:            results.Dkim,
		Dmarc:           results.Dmarc,
		Handoff:         results.Handoff,
		InboundId:       email.Mail.InboundId,
		RawBlobKey:      email.Mail.RawBlobKey,
	}
}
