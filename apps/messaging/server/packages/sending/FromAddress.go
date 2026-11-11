package sending

import (
	"slices"

	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
)

type FromAddress struct {
	directory core.Directory
}

func NewFromAddress(directory core.Directory) FromAddress {
	return FromAddress{directory: directory}
}

func (from FromAddress) of(caller oidc.Caller, chosen string, conversation core.Conversation) (string, error) {
	return from.route(chosen)(caller, chosen, conversation)
}

func (from FromAddress) route(chosen string) func(oidc.Caller, string, core.Conversation) (string, error) {
	if chosen == "" {
		return from.standard
	}
	return from.checked
}

func (from FromAddress) standard(caller oidc.Caller, chosen string, conversation core.Conversation) (string, error) {
	return from.standardAddress(caller, conversation), nil
}

func (FromAddress) standardAddress(caller oidc.Caller, conversation core.Conversation) string {
	if conversation.Kind == (core.ConversationKinds{}).Channel() && conversation.EmailAddress != "" {
		return conversation.EmailAddress
	}
	return Lists{}.address(caller.Email)
}

func (from FromAddress) checked(caller oidc.Caller, chosen string, conversation core.Conversation) (string, error) {
	entry, found, err := from.directory.Address(caller.DomainId, chosen)
	if err != nil {
		return "", Failures{}.directory(err)
	}
	return chosen, from.refusal(from.owned(caller, chosen, entry, found))
}

func (from FromAddress) owned(caller oidc.Caller, chosen string, entry core.AddressEntry, found bool) bool {
	return chosen == Lists{}.address(caller.Email) || (found && slices.Contains(from.ownedKinds(), entry.Kind) && entry.TargetId == caller.MemberId)
}

func (FromAddress) refusal(owned bool) error {
	if owned {
		return nil
	}
	return Failures{}.invalid("the From address must be one of your own addresses", "fromAddress")
}

func (FromAddress) ownedKinds() []string {
	return []string{"member", "alias"}
}
