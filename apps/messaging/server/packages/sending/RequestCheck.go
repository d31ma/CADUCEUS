package sending

import (
	"slices"
	"strings"
	"unicode/utf8"

	"mesail.dev/packages/core"
	kit "mesail.dev/packages/recipients"
)

type RequestCheck struct{}

func (check RequestCheck) Normalized(request core.SendRequest) core.SendRequest {
	normalized := request
	normalized.Recipients = check.normalizedRecipients(request.Recipients)
	normalized.Email.FromAddress = Lists{}.address(request.Email.FromAddress)
	return normalized
}

func (check RequestCheck) Check(request core.SendRequest) error {
	refusals := slices.Concat(
		check.clientMessageId(request.ClientMessageId),
		check.bodyLength(request.Body),
		check.bodyPresent(request),
		check.attachmentCount(request.AttachmentIds),
		check.attachmentRefusals(request.AttachmentIds),
		check.recipientCount(request.Recipients),
		check.recipientRefusals(request.Recipients),
		check.duplicates(request.Recipients),
		check.recipientsNeeded(request),
		check.alsoToConversation(request),
	)
	return Failures{}.first(refusals)
}

func (check RequestCheck) normalizedRecipients(choices []core.RecipientChoice) []core.RecipientChoice {
	normalized := []core.RecipientChoice{}
	for _, choice := range choices {
		choice.Address = Lists{}.address(choice.Address)
		normalized = append(normalized, choice)
	}
	return normalized
}

func (check RequestCheck) clientMessageId(clientMessageId string) []error {
	length := utf8.RuneCountInString(clientMessageId)
	if length >= 1 && length <= check.clientMessageIdLimit() {
		return nil
	}
	return []error{Failures{}.invalid("clientMessageId must have 1 to 200 characters", "clientMessageId")}
}

func (check RequestCheck) bodyLength(body string) []error {
	if utf8.RuneCountInString(body) <= check.bodyLimit() {
		return nil
	}
	return []error{Failures{}.invalid("the body may have at most 40000 characters", check.bodyField())}
}

func (check RequestCheck) bodyPresent(request core.SendRequest) []error {
	if strings.TrimSpace(request.Body) != "" || len(request.AttachmentIds) > 0 {
		return nil
	}
	return []error{Failures{}.invalid("the message needs a body or an attachment", check.bodyField())}
}

func (check RequestCheck) attachmentCount(attachmentIds []string) []error {
	if len(attachmentIds) <= check.attachmentLimit() {
		return nil
	}
	return []error{Failures{}.invalid("a message may have at most 10 attachments", check.attachmentField())}
}

func (check RequestCheck) attachmentRefusals(attachmentIds []string) []error {
	refusals := []error{}
	for _, attachmentId := range attachmentIds {
		refusals = append(refusals, check.attachmentRefusal(attachmentId)...)
	}
	return refusals
}

func (check RequestCheck) attachmentRefusal(attachmentId string) []error {
	if strings.HasPrefix(attachmentId, check.attachmentPrefix()) {
		return nil
	}
	return []error{Failures{}.invalid("an attachment id must start with att_", check.attachmentField())}
}

func (check RequestCheck) recipientCount(choices []core.RecipientChoice) []error {
	if len(choices) <= check.recipientLimit() {
		return nil
	}
	return []error{Failures{}.recipients("a message may have at most 20 recipients")}
}

func (check RequestCheck) recipientRefusals(choices []core.RecipientChoice) []error {
	refusals := []error{}
	for _, choice := range choices {
		refusals = append(refusals, check.recipientShape(choice)...)
		refusals = append(refusals, check.blindCopyShape(choice)...)
	}
	return refusals
}

func (check RequestCheck) recipientShape(choice core.RecipientChoice) []error {
	if choice.Address != "" && slices.Contains(check.roles(), choice.Role) && slices.Contains(check.deliveries(), choice.Delivery) && slices.Contains(check.kinds(), choice.ExpectedKind) {
		return nil
	}
	return []error{Failures{}.recipients("each recipient needs an address, a role, a delivery and an expected kind")}
}

func (check RequestCheck) blindCopyShape(choice core.RecipientChoice) []error {
	if choice.Role != (core.RecipientRoles{}).Bcc() || choice.Delivery == string(kit.Deliveries{}.Email()) {
		return nil
	}
	return []error{Failures{}.recipients("a Bcc recipient gets email only")}
}

func (check RequestCheck) duplicates(choices []core.RecipientChoice) []error {
	seen := map[string]bool{}
	refusals := []error{}
	for _, choice := range choices {
		refusals = append(refusals, check.duplicate(choice.Address, seen)...)
	}
	return refusals
}

func (check RequestCheck) duplicate(address string, seen map[string]bool) []error {
	if !seen[address] {
		seen[address] = true
		return nil
	}
	return []error{Failures{}.recipients("each address may appear only once")}
}

func (check RequestCheck) recipientsNeeded(request core.SendRequest) []error {
	if request.ConversationId != "" || len(request.Recipients) > 0 {
		return nil
	}
	return []error{Failures{}.recipients("a new conversation needs recipients")}
}

func (check RequestCheck) alsoToConversation(request core.SendRequest) []error {
	if !request.AlsoToConversation || request.ThreadRootId != "" {
		return nil
	}
	return []error{Failures{}.invalid("alsoToConversation is allowed only with threadRootId", "alsoToConversation")}
}

func (RequestCheck) roles() []string {
	roles := core.RecipientRoles{}
	return []string{roles.To(), roles.Cc(), roles.Bcc()}
}

func (RequestCheck) deliveries() []string {
	deliveries := kit.Deliveries{}
	return []string{string(deliveries.Chat()), string(deliveries.Email()), string(deliveries.Both())}
}

func (RequestCheck) kinds() []string {
	kinds := kit.Kinds{}
	return []string{string(kinds.Member()), string(kinds.Channel()), string(kinds.OtherDomain()), string(kinds.External())}
}

func (RequestCheck) clientMessageIdLimit() int {
	return 200
}

func (RequestCheck) bodyLimit() int {
	return 40000
}

func (RequestCheck) attachmentLimit() int {
	return 10
}

func (RequestCheck) recipientLimit() int {
	return 20
}

func (RequestCheck) attachmentPrefix() string {
	return "att_"
}

func (RequestCheck) bodyField() string {
	return "body"
}

func (RequestCheck) attachmentField() string {
	return "attachmentIds"
}
