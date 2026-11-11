package search

import (
	"strings"
	"time"

	"mesail.dev/packages/events"
	"mesail.dev/packages/textindex"
)

type DocumentBuilder struct{}

func (builder DocumentBuilder) FromCreated(domainId string, message events.MessageCreated) (Document, string, error) {
	createdAt, err := builder.instant(message.CreatedAt, "createdAt")
	if err != nil {
		return Document{}, "", err
	}
	document := Document{
		DomainId:       domainId,
		MessageId:      message.MessageId,
		ConversationId: message.ConversationId,
		AuthorMemberId: message.AuthorMemberId,
		AuthorAddress:  strings.ToLower(message.AuthorAddress),
		AuthorName:     message.AuthorDisplayName,
		Text:           message.Body,
		Subject:        message.Subject,
		FileNames:      builder.names(message.FileNames),
		HasAttachment:  len(message.AttachmentIds) > 0,
		IsEmail:        message.IsEmail,
		CreatedAt:      builder.Stamp(createdAt),
	}
	return builder.withTerms(document), builder.Month(createdAt), builder.required(message)
}

func (builder DocumentBuilder) Edited(document Document, message events.MessageEdited) (Document, error) {
	editedAt, err := builder.instant(message.EditedAt, "editedAt")
	document.Text = message.Body
	document.EditedAt = builder.Stamp(editedAt)
	return builder.withTerms(document), err
}

func (DocumentBuilder) Tombstone(document Document) Document {
	return Document{DomainId: document.DomainId, MessageId: document.MessageId, ConversationId: document.ConversationId, CreatedAt: document.CreatedAt, FileNames: []string{}, Terms: []string{}, Deleted: true}
}

func (builder DocumentBuilder) Stamp(instant time.Time) string {
	return instant.UTC().Format(builder.stampLayout())
}

func (builder DocumentBuilder) Month(instant time.Time) string {
	return instant.UTC().Format(builder.monthLayout())
}

func (builder DocumentBuilder) withTerms(document Document) Document {
	parts := append([]string{document.Text, document.Subject, document.AuthorName, document.AuthorAddress}, document.FileNames...)
	document.Terms = textindex.Tokenizer{}.Terms(strings.Join(parts, builder.space()))
	return document
}

func (DocumentBuilder) instant(text string, field string) (time.Time, error) {
	instant, err := time.Parse(time.RFC3339Nano, text)
	if err != nil {
		return time.Time{}, InvalidRequest{Message: field + " is not an RFC 3339 time", Field: field}
	}
	return instant, nil
}

func (DocumentBuilder) required(message events.MessageCreated) error {
	if message.MessageId == "" || message.ConversationId == "" {
		return InvalidRequest{Message: "messageId and conversationId are required", Field: "payload"}
	}
	return nil
}

func (DocumentBuilder) names(fileNames []string) []string {
	if fileNames == nil {
		return []string{}
	}
	return fileNames
}

func (DocumentBuilder) stampLayout() string {
	return "2006-01-02T15:04:05.000000Z"
}

func (DocumentBuilder) monthLayout() string {
	return "200601"
}

func (DocumentBuilder) space() string {
	return " "
}
