import (
	"cmp"
	"maps"
	"slices"
	"strings"
)

// @Service
type ConversationMessageService struct {
	// @Needs
	reactions *ConversationReactionService
}

func (service *ConversationMessageService) Combine(newer []Message, older []Message) []Message {
	known := service.idsOf(newer)
	return append(slices.Clone(newer), service.unknown(older, known)...)
}

func (service *ConversationMessageService) Replace(messages []Message, changed Message) []Message {
	return service.replaced(messages, changed)
}

func (service *ConversationMessageService) Find(messages []Message, messageId string) Message {
	return service.messageAt(messages, service.indexOf(messages, messageId))
}

func (service *ConversationMessageService) Newest(messages []Message) string {
	return service.firstId(messages)
}

func (service *ConversationMessageService) Subject(messages []Message) string {
	return service.subjectAt(messages, slices.IndexFunc(messages, service.isEmail))
}

func (service *ConversationMessageService) DeleteWarning(message Message) string {
	return service.warningFor(service.emailRecipients(message.Deliveries))
}

func (service *ConversationMessageService) Toggled(values []string, value string) []string {
	return service.toggled(values, value)
}

func (service *ConversationMessageService) WithSender(senders map[string][]string, domainId string, address string) map[string][]string {
	updated := map[string][]string{}
	maps.Copy(updated, senders)
	updated[domainId] = service.added(updated[domainId], strings.ToLower(strings.TrimSpace(address)))
	return updated
}

func (service *ConversationMessageService) ReactionChoices(messages []Message, messageId string) []MessageReaction {
	return service.reactions.Choices(service.Find(messages, messageId).MyReactions)
}

func (service *ConversationMessageService) HasReaction(messages []Message, messageId string, name string) bool {
	return slices.Contains(service.Find(messages, messageId).MyReactions, name)
}

func (service *ConversationMessageService) Announcement(message Message, myAddress string) string {
	return service.announcementUnlessMine(message, myAddress)
}

func (service *ConversationMessageService) idsOf(messages []Message) []string {
	ids := []string{}
	for _, message := range messages {
		ids = append(ids, message.MessageId)
	}
	return ids
}

func (service *ConversationMessageService) unknown(messages []Message, known []string) []Message {
	return slices.DeleteFunc(slices.Clone(messages), service.knownTo(known))
}

func (service *ConversationMessageService) knownTo(known []string) func(Message) bool {
	return func(message Message) bool { return slices.Contains(known, message.MessageId) }
}

func (service *ConversationMessageService) replaced(messages []Message, changed Message) []Message {
	updated := []Message{}
	for _, message := range messages {
		updated = append(updated, service.newerOf(message, changed))
	}
	return updated
}

func (service *ConversationMessageService) newerOf(message Message, changed Message) Message {
	if message.MessageId == changed.MessageId {
		return changed
	}
	return message
}

func (service *ConversationMessageService) indexOf(messages []Message, messageId string) int {
	return slices.Index(service.idsOf(messages), messageId)
}

func (service *ConversationMessageService) messageAt(messages []Message, index int) Message {
	if index < 0 {
		return Message{}
	}
	return messages[index]
}

func (service *ConversationMessageService) firstId(messages []Message) string {
	if len(messages) == 0 {
		return ""
	}
	return messages[0].MessageId
}

func (service *ConversationMessageService) isEmail(message Message) bool {
	return message.Email != nil
}

func (service *ConversationMessageService) subjectAt(messages []Message, index int) string {
	if index < 0 {
		return ""
	}
	return messages[index].Email.Subject
}

func (service *ConversationMessageService) emailRecipients(deliveries []DeliveryStatus) []string {
	recipients := []string{}
	for _, delivery := range deliveries {
		recipients = append(recipients, service.recipientByEmail(delivery))
	}
	return slices.DeleteFunc(recipients, service.isEmpty)
}

func (service *ConversationMessageService) recipientByEmail(delivery DeliveryStatus) string {
	if delivery.Channel != service.emailChannel() {
		return ""
	}
	return delivery.Recipient
}

func (service *ConversationMessageService) warningFor(recipients []string) string {
	if len(recipients) == 0 {
		return ""
	}
	return strings.Join(recipients, ", ") + " already got this as an email. Email cannot be recalled."
}

func (service *ConversationMessageService) toggled(values []string, value string) []string {
	if slices.Contains(values, value) {
		return slices.DeleteFunc(slices.Clone(values), service.equalTo(value))
	}
	return append(slices.Clone(values), value)
}

func (service *ConversationMessageService) added(values []string, value string) []string {
	if slices.Contains(values, value) {
		return values
	}
	return append(slices.Clone(values), value)
}

func (service *ConversationMessageService) equalTo(value string) func(string) bool {
	return func(text string) bool { return text == value }
}

func (service *ConversationMessageService) isEmpty(text string) bool {
	return text == ""
}

func (service *ConversationMessageService) announcementUnlessMine(message Message, myAddress string) string {
	if service.anyTrue(message.MessageId == "", strings.EqualFold(message.Author.Address, myAddress)) {
		return ""
	}
	return "New message from " + cmp.Or(message.Author.DisplayName, message.Author.Address)
}

func (service *ConversationMessageService) anyTrue(conditions ...bool) bool {
	return slices.Contains(conditions, true)
}

func (service *ConversationMessageService) emailChannel() string {
	return "email"
}
