import (
	"regexp"
	"slices"
	"strings"
)

// @Service
type ComposerRecipientService struct {
	Recipients []Recipient
	Problem    string
	// @Needs
	resolve *ComposerResolveService
	// @Needs
	words *ComposerWordsService
	// @Needs
	auth *ComposerAuthService
}

func (service *ComposerRecipientService) Add(text string, role string) {
	service.Recipients = append(service.Recipients, service.newChips(service.addressesIn(text), role)...)
	service.ResolvePending()
}

func (service *ComposerRecipientService) ResolvePending() {
	resolved, failure := service.resolveAny(service.addressesInState("checking"))
	service.Problem = service.problemOf(failure)
	service.Recipients = service.withResolved(resolved)
}

func (service *ComposerRecipientService) Remove(address string, role string) {
	service.Recipients = slices.DeleteFunc(slices.Clone(service.Recipients), service.matching(address, role))
}

func (service *ComposerRecipientService) RemoveLast(role string) {
	service.Recipients = service.withoutLast(role)
}

func (service *ComposerRecipientService) Choose(address string, role string, delivery string) {
	service.Recipients = service.updated(address, role, service.choosing(delivery))
}

func (service *ComposerRecipientService) ToggleMenu(address string, role string) {
	service.CloseMenusExcept(address, role)
	service.Recipients = service.updated(address, role, service.toggled)
}

func (service *ComposerRecipientService) CloseMenus() {
	service.CloseMenusExcept("", "")
}

func (service *ComposerRecipientService) CloseMenusExcept(address string, role string) {
	service.Recipients = service.mapped(service.closingExcept(address, role))
}

func (service *ComposerRecipientService) Refresh() []RecipientChange {
	before := slices.Clone(service.Recipients)
	resolved, failure := service.resolveAny(service.addressesInState("ready"))
	service.Problem = service.problemOf(failure)
	service.Recipients = service.withRefreshed(resolved)
	return service.changesFrom(before)
}

func (service *ComposerRecipientService) Clear() {
	service.Recipients = []Recipient{}
	service.Problem = ""
}

func (service *ComposerRecipientService) InRole(role string) []Recipient {
	return slices.DeleteFunc(slices.Clone(service.Recipients), service.notInRole(role))
}

func (service *ComposerRecipientService) Choices() []RecipientChoice {
	return service.choicesOf(service.Recipients)
}

func (service *ComposerRecipientService) CountInState(state string) int {
	return len(service.addressesInState(state))
}

func (service *ComposerRecipientService) Summary() []string {
	return service.words.Summary(service.Recipients)
}

func (service *ComposerRecipientService) SendLabel() string {
	return service.words.SendLabel(service.Recipients)
}

func (service *ComposerRecipientService) EmailMode() bool {
	return service.words.EmailMode(service.Recipients)
}

func (service *ComposerRecipientService) ModeText() string {
	return service.words.ModeText(service.Recipients, service.auth.DomainId())
}

func (service *ComposerRecipientService) choosing(delivery string) func(Recipient) Recipient {
	return func(recipient Recipient) Recipient {
		return service.resolve.Labelled(service.withDelivery(recipient, delivery))
	}
}

func (service *ComposerRecipientService) toggled(recipient Recipient) Recipient {
	recipient.MenuOpen = !recipient.MenuOpen
	return recipient
}

func (service *ComposerRecipientService) closingExcept(address string, role string) func(Recipient) Recipient {
	return func(recipient Recipient) Recipient {
		recipient.MenuOpen = recipient.MenuOpen && service.matching(address, role)(recipient)
		return recipient
	}
}

func (service *ComposerRecipientService) notInRole(role string) func(Recipient) bool {
	return func(recipient Recipient) bool {
		return recipient.Role != role
	}
}

func (service *ComposerRecipientService) choicesOf(recipients []Recipient) []RecipientChoice {
	choices := []RecipientChoice{}
	for _, recipient := range recipients {
		choices = append(choices, RecipientChoice{Address: recipient.Address, Role: recipient.Role, Delivery: recipient.Delivery, ExpectedKind: recipient.Kind})
	}
	return choices
}

func (service *ComposerRecipientService) resolveAny(addresses []string) ([]ResolvedRecipient, error) {
	if len(addresses) == 0 {
		return []ResolvedRecipient{}, nil
	}
	return service.resolve.Resolve(addresses)
}

func (service *ComposerRecipientService) newChips(addresses []string, role string) []Recipient {
	chips := []Recipient{}
	for _, address := range addresses {
		chips = service.appendNew(chips, address, role)
	}
	return chips
}

func (service *ComposerRecipientService) appendNew(chips []Recipient, address string, role string) []Recipient {
	if slices.ContainsFunc(service.Recipients, service.matching(address, role)) {
		return chips
	}
	return append(chips, service.chipFor(address, role))
}

func (service *ComposerRecipientService) chipFor(address string, role string) Recipient {
	if !service.addressPattern().MatchString(address) {
		return service.resolve.Invalid(address, role)
	}
	return service.resolve.Checking(address, role)
}

func (service *ComposerRecipientService) addressesIn(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(letter rune) bool {
		return strings.ContainsRune(" ,;\n\t", letter)
	})
}

func (service *ComposerRecipientService) addressesInState(state string) []string {
	addresses := []string{}
	for _, recipient := range service.Recipients {
		addresses = service.appendAddressWhen(addresses, recipient.Address, recipient.State == state)
	}
	return addresses
}

func (service *ComposerRecipientService) appendAddressWhen(addresses []string, address string, wanted bool) []string {
	if !wanted || slices.Contains(addresses, address) {
		return addresses
	}
	return append(addresses, address)
}

func (service *ComposerRecipientService) withResolved(resolved []ResolvedRecipient) []Recipient {
	return service.mapped(func(recipient Recipient) Recipient {
		return service.resolvedWhenChecking(recipient, service.byAddress(resolved))
	})
}

func (service *ComposerRecipientService) resolvedWhenChecking(recipient Recipient, resolved map[string]ResolvedRecipient) Recipient {
	found, known := resolved[recipient.Address]
	if recipient.State != "checking" || !known {
		return recipient
	}
	return service.resolve.RecipientOf(found, recipient.Role, found.DefaultDelivery)
}

func (service *ComposerRecipientService) withRefreshed(resolved []ResolvedRecipient) []Recipient {
	return service.mapped(func(recipient Recipient) Recipient {
		return service.refreshedWhenKnown(recipient, service.byAddress(resolved))
	})
}

func (service *ComposerRecipientService) refreshedWhenKnown(recipient Recipient, resolved map[string]ResolvedRecipient) Recipient {
	found, known := resolved[recipient.Address]
	if !known {
		return recipient
	}
	return service.resolve.RecipientOf(found, recipient.Role, recipient.Delivery)
}

func (service *ComposerRecipientService) changesFrom(before []Recipient) []RecipientChange {
	changes := []RecipientChange{}
	for index, old := range before {
		changes = service.appendChangeWhen(changes, old, service.Recipients[index])
	}
	return changes
}

func (service *ComposerRecipientService) appendChangeWhen(changes []RecipientChange, old Recipient, fresh Recipient) []RecipientChange {
	if old.Kind == fresh.Kind && old.Delivery == fresh.Delivery {
		return changes
	}
	sentence := fresh.DisplayName + " now gets: " + fresh.Label + "."
	return append(changes, RecipientChange{Address: fresh.Address, DisplayName: fresh.DisplayName, Before: old.Label, After: fresh.Label, Sentence: sentence})
}

func (service *ComposerRecipientService) byAddress(resolved []ResolvedRecipient) map[string]ResolvedRecipient {
	found := map[string]ResolvedRecipient{}
	for _, recipient := range resolved {
		found[strings.ToLower(recipient.Address)] = recipient
	}
	return found
}

func (service *ComposerRecipientService) withDelivery(recipient Recipient, delivery string) Recipient {
	if !service.allowed(recipient)[delivery] {
		return recipient
	}
	recipient.Delivery = delivery
	recipient.MenuOpen = false
	return recipient
}

func (service *ComposerRecipientService) allowed(recipient Recipient) map[string]bool {
	return map[string]bool{"chat": recipient.ChatAllowed && recipient.Kind == "member", "email": recipient.Kind != "channel", "both": recipient.Kind == "member"}
}

func (service *ComposerRecipientService) withoutLast(role string) []Recipient {
	inRole := service.InRole(role)
	if len(inRole) == 0 {
		return service.Recipients
	}
	last := inRole[len(inRole)-1]
	return slices.DeleteFunc(slices.Clone(service.Recipients), service.matching(last.Address, role))
}

func (service *ComposerRecipientService) updated(address string, role string, change func(Recipient) Recipient) []Recipient {
	return service.mapped(func(recipient Recipient) Recipient {
		return service.changedWhenMatching(recipient, address, role, change)
	})
}

func (service *ComposerRecipientService) changedWhenMatching(recipient Recipient, address string, role string, change func(Recipient) Recipient) Recipient {
	if !service.matching(address, role)(recipient) {
		return recipient
	}
	return change(recipient)
}

func (service *ComposerRecipientService) mapped(change func(Recipient) Recipient) []Recipient {
	recipients := []Recipient{}
	for _, recipient := range service.Recipients {
		recipients = append(recipients, change(recipient))
	}
	return recipients
}

func (service *ComposerRecipientService) matching(address string, role string) func(Recipient) bool {
	return func(recipient Recipient) bool {
		return recipient.Address == address && recipient.Role == role
	}
}

func (service *ComposerRecipientService) problemOf(failure error) string {
	if failure != nil {
		return "resolveFailed"
	}
	return ""
}

func (service *ComposerRecipientService) addressPattern() *regexp.Regexp {
	return regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
}

func (service *ComposerRecipientService) SenderAddress() string {
	return service.auth.Address()
}

func (service *ComposerRecipientService) Find(address string) Recipient {
	return service.recipientAt(slices.IndexFunc(service.Recipients, service.withAddress(address)))
}

func (service *ComposerRecipientService) withAddress(address string) func(Recipient) bool {
	return func(recipient Recipient) bool {
		return recipient.Address == address
	}
}

func (service *ComposerRecipientService) recipientAt(index int) Recipient {
	if index < 0 {
		return Recipient{}
	}
	return service.Recipients[index]
}
