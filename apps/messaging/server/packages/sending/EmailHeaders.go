package sending

import (
	"slices"
	"strings"
	"time"

	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
)

type EmailHeaders struct {
	previous PreviousEmail
	from     FromAddress
	now      time.Time
}

func NewEmailHeaders(stores core.Stores, directory core.Directory, now time.Time) EmailHeaders {
	return EmailHeaders{previous: NewPreviousEmail(stores.Messages), from: NewFromAddress(directory), now: now}
}

func (headers EmailHeaders) Build(caller oidc.Caller, draft Draft) (Draft, error) {
	return headers.route(len(DeliveryRows{}.email(draft.Rows)))(caller, draft)
}

func (headers EmailHeaders) route(emailCount int) func(oidc.Caller, Draft) (Draft, error) {
	if emailCount == 0 {
		return headers.chatOnly
	}
	return headers.email
}

func (EmailHeaders) chatOnly(caller oidc.Caller, draft Draft) (Draft, error) {
	return draft, nil
}

func (headers EmailHeaders) email(caller oidc.Caller, draft Draft) (Draft, error) {
	conversation := draft.Placement.Conversation
	from, err := headers.from.of(caller, draft.Request.Email.FromAddress, conversation)
	if err != nil {
		return draft, err
	}
	earlier, found, err := headers.previous.find(caller.DomainId, core.ConversationIds{}.Wire(conversation), draft.Thread, headers.now)
	if err != nil {
		return draft, err
	}
	subject, err := headers.subject(draft.Request.Email.Subject, headers.baseSubject(earlier, found, conversation))
	if err != nil {
		return draft, err
	}
	draft.IsEmail = true
	draft.Email = headers.meta(draft.Rows, from, subject, earlier)
	return draft, nil
}

func (headers EmailHeaders) meta(rows []core.DeliveryRecord, from string, subject string, earlier core.Message) core.EmailMeta {
	roles := core.RecipientRoles{}
	emailRows := DeliveryRows{}.email(rows)
	return core.EmailMeta{
		InReplyTo:  earlier.Email.MessageIdHeader,
		References: headers.references(earlier),
		Subject:    subject,
		From:       from,
		To:         DeliveryRows{}.addresses(emailRows, roles.To()),
		Cc:         slices.Concat(DeliveryRows{}.addresses(emailRows, roles.Cc()), DeliveryRows{}.chatOnlyMembers(rows)),
	}
}

func (headers EmailHeaders) references(earlier core.Message) []string {
	if earlier.Email.MessageIdHeader == "" {
		return []string{}
	}
	return Lists{}.last(append(Lists{}.copied(earlier.Email.References), earlier.Email.MessageIdHeader), headers.referenceLimit())
}

func (headers EmailHeaders) subject(given string, base string) (string, error) {
	chosen := headers.chosenSubject(strings.TrimSpace(given), headers.replySubject(base))
	return chosen, Failures{}.subjectRefusal(chosen)
}

func (EmailHeaders) baseSubject(earlier core.Message, found bool, conversation core.Conversation) string {
	if found && earlier.Email.Subject != "" {
		return earlier.Email.Subject
	}
	return conversation.Subject
}

func (headers EmailHeaders) replySubject(base string) string {
	if base == "" || strings.HasPrefix(strings.ToLower(base), headers.replyMarker()) {
		return base
	}
	return headers.replyPrefix() + base
}

func (EmailHeaders) chosenSubject(given string, reply string) string {
	if given != "" {
		return given
	}
	return reply
}

func (EmailHeaders) replyMarker() string {
	return "re:"
}

func (EmailHeaders) replyPrefix() string {
	return "Re: "
}

func (EmailHeaders) referenceLimit() int {
	return 20
}
