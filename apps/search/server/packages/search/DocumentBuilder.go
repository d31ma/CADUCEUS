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
	labels := DocumentBudget{}.LabelCharacters()
	document := Document{
		DomainId:       domainId,
		MessageId:      message.MessageId,
		ConversationId: message.ConversationId,
		AuthorMemberId: message.AuthorMemberId,
		AuthorAddress:  builder.cut(strings.ToLower(message.AuthorAddress), labels),
		AuthorName:     builder.cut(message.AuthorDisplayName, labels),
		Text:           message.Body,
		Subject:        builder.cut(message.Subject, labels),
		FileNames:      builder.names(message.FileNames),
		HasAttachment:  len(message.AttachmentIds) > 0,
		IsEmail:        message.IsEmail,
		CreatedAt:      builder.Stamp(createdAt),
	}
	return builder.sized(document), builder.Month(createdAt), builder.required(message)
}

func (builder DocumentBuilder) Edited(document Document, message events.MessageEdited) (Document, error) {
	editedAt, err := builder.instant(message.EditedAt, "editedAt")
	document.Text = message.Body
	document.Truncated = false
	document.EditedAt = builder.Stamp(editedAt)
	return builder.sized(document), err
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

func (builder DocumentBuilder) sized(document Document) Document {
	return builder.fitted(builder.withTerms(builder.capped(document)))
}

func (DocumentBuilder) capped(document Document) Document {
	characters := []rune(document.Text)
	limit := DocumentBudget{}.TextCharacters()
	if len(characters) <= limit {
		return document
	}
	document.Text = string(characters[:limit])
	document.Truncated = true
	return document
}

func (builder DocumentBuilder) fitted(document Document) Document {
	budget := DocumentBudget{}
	for budget.Over(document) && document.Text != "" {
		document = builder.shorter(document)
	}
	return document
}

func (builder DocumentBuilder) shorter(document Document) Document {
	budget := DocumentBudget{}
	characters := []rune(document.Text)
	document.Text = string(characters[:len(characters)*budget.MaxBytes()/budget.Size(document)])
	document.Truncated = true
	return builder.withTerms(document)
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

func (builder DocumentBuilder) names(fileNames []string) []string {
	budget := DocumentBudget{}
	names := []string{}
	for _, name := range fileNames[:min(len(fileNames), budget.MaxFileNames())] {
		names = append(names, builder.cut(name, budget.FileNameCharacters()))
	}
	return names
}

func (DocumentBuilder) cut(text string, limit int) string {
	characters := []rune(text)
	return string(characters[:min(len(characters), limit)])
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
