package sending

import (
	"slices"

	"mesail.dev/packages/core"
	kit "mesail.dev/packages/recipients"
)

type RecipientCheck struct {
	directory core.Directory
}

func NewRecipientCheck(directory core.Directory) RecipientCheck {
	return RecipientCheck{directory: directory}
}

func (check RecipientCheck) Checked(authorization string, request core.SendRequest) ([]Recipient, error) {
	answers, err := check.resolver(request.Recipients)(authorization, request.Recipients)
	if err != nil {
		return nil, err
	}
	fresh := check.indexed(answers)
	err = check.changedRefusal(check.changed(request.Recipients, fresh))
	if err != nil {
		return nil, err
	}
	chosen := check.merged(request.Recipients, fresh)
	refusals := slices.Concat(check.deliveryRefusals(chosen), check.channelPlacement(request.ConversationId, chosen))
	return chosen, Failures{}.first(refusals)
}

func (check RecipientCheck) resolver(choices []core.RecipientChoice) func(string, []core.RecipientChoice) ([]core.Resolved, error) {
	if len(choices) == 0 {
		return check.nobody
	}
	return check.resolve
}

func (RecipientCheck) nobody(authorization string, choices []core.RecipientChoice) ([]core.Resolved, error) {
	return []core.Resolved{}, nil
}

func (check RecipientCheck) resolve(authorization string, choices []core.RecipientChoice) ([]core.Resolved, error) {
	answers, err := check.directory.Resolve(authorization, check.addresses(choices))
	if err != nil {
		return nil, Failures{}.directory(err)
	}
	return answers, nil
}

func (RecipientCheck) addresses(choices []core.RecipientChoice) []string {
	addresses := []string{}
	for _, choice := range choices {
		addresses = append(addresses, choice.Address)
	}
	return addresses
}

func (RecipientCheck) indexed(answers []core.Resolved) map[string]core.Resolved {
	fresh := map[string]core.Resolved{}
	for _, answer := range answers {
		fresh[Lists{}.address(answer.Address)] = answer
	}
	return fresh
}

func (check RecipientCheck) changed(choices []core.RecipientChoice, fresh map[string]core.Resolved) []string {
	changed := []string{}
	for _, choice := range choices {
		changed = append(changed, check.ifChanged(choice, fresh)...)
	}
	return changed
}

func (RecipientCheck) ifChanged(choice core.RecipientChoice, fresh map[string]core.Resolved) []string {
	answer, found := fresh[choice.Address]
	if found && answer.Kind == choice.ExpectedKind {
		return nil
	}
	return []string{choice.Address}
}

func (RecipientCheck) changedRefusal(changed []string) error {
	if len(changed) == 0 {
		return nil
	}
	return &core.RecipientsChanged{Addresses: changed}
}

func (RecipientCheck) merged(choices []core.RecipientChoice, fresh map[string]core.Resolved) []Recipient {
	chosen := []Recipient{}
	for _, choice := range choices {
		answer := fresh[choice.Address]
		chosen = append(chosen, Recipient{Address: choice.Address, Role: choice.Role, Delivery: choice.Delivery, Kind: answer.Kind, MemberId: answer.MemberId, DisplayName: answer.DisplayName, ChannelId: answer.ChannelId})
	}
	return chosen
}

func (check RecipientCheck) deliveryRefusals(chosen []Recipient) []error {
	refusals := []error{}
	for _, recipient := range chosen {
		allowed := check.allowed(recipient)
		refusals = append(refusals, check.channelDeliveryRefusal(recipient, allowed)...)
		refusals = append(refusals, check.chatRefusal(recipient, allowed)...)
	}
	return refusals
}

func (RecipientCheck) allowed(recipient Recipient) bool {
	return kit.Policy{}.Check(kit.Kind(recipient.Kind), kit.Delivery(recipient.Delivery)) == nil
}

func (check RecipientCheck) channelDeliveryRefusal(recipient Recipient, allowed bool) []error {
	if allowed || recipient.Kind != check.channelKind() {
		return nil
	}
	return []error{Failures{}.recipients("a channel gets chat only: " + recipient.Address)}
}

func (check RecipientCheck) chatRefusal(recipient Recipient, allowed bool) []error {
	if allowed || recipient.Kind == check.channelKind() {
		return nil
	}
	return []error{&core.ChatNotAllowed{Address: recipient.Address}}
}

func (check RecipientCheck) channelPlacement(conversationId string, chosen []Recipient) []error {
	return check.channelAlone(conversationId, chosen, check.channelCount(chosen))
}

func (check RecipientCheck) channelCount(chosen []Recipient) int {
	count := 0
	for _, recipient := range chosen {
		count += Lists{}.oneIf(recipient.Kind == check.channelKind())
	}
	return count
}

func (RecipientCheck) channelAlone(conversationId string, chosen []Recipient, channelCount int) []error {
	if channelCount == 0 || (len(chosen) == 1 && chosen[0].Role == (core.RecipientRoles{}).To() && conversationId == "") {
		return nil
	}
	return []error{Failures{}.recipients("a channel must be the only recipient of a new message, in To")}
}

func (RecipientCheck) channelKind() string {
	return string(kit.Kinds{}.Channel())
}
