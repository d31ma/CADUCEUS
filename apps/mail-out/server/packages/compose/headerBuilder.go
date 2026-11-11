package compose

import (
	"time"
)

type headerBuilder struct {
	names   fieldNames
	encoder encoder
	list    referenceList
}

func (builder headerBuilder) build(message Message) []string {
	names := builder.names
	lines := []string{builder.line(names.From(), builder.encoder.address(message.From))}
	lines = append(lines, builder.addressHeader(names.To(), message.To)...)
	lines = append(lines, builder.addressHeader(names.Cc(), message.Cc)...)
	lines = append(lines, builder.addressHeader(names.ReplyTo(), message.ReplyTo)...)
	lines = append(lines,
		builder.line(names.Subject(), builder.encoder.words(message.Subject)),
		builder.line(names.Date(), builder.dateOf(message.Date).Format(time.RFC1123Z)),
	)
	lines = append(lines, builder.idHeader(names.MessageId(), []string{message.MessageId})...)
	lines = append(lines, builder.idHeader(names.InReplyTo(), []string{message.InReplyTo})...)
	lines = append(lines, builder.idHeader(names.References(), builder.list.capped(message.References))...)
	return append(lines, "MIME-Version: 1.0")
}

func (builder headerBuilder) line(name string, value string) string {
	return folder{}.fold(name + ": " + value)
}

func (builder headerBuilder) addressHeader(name string, addresses []Address) []string {
	if len(addresses) == 0 {
		return nil
	}
	return []string{builder.line(name, builder.encoder.addressList(addresses))}
}

func (builder headerBuilder) idHeader(name string, ids []string) []string {
	value := builder.list.bracketed(builder.list.unique(ids))
	if value == "" {
		return nil
	}
	return []string{builder.line(name, value)}
}

func (headerBuilder) dateOf(date time.Time) time.Time {
	if date.IsZero() {
		return time.Now()
	}
	return date
}
