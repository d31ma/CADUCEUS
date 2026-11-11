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
	fresh := service.notYetAdded(service.addressesIn(text), role)
	service.Recipients = append(service.Recipients, service.chipsFor(fresh, role)...)
	service.ResolvePending()
}

func (service *ComposerRecipientService) ResolvePending() {
	resolved, failure := service.resolveAny(service.addressesInState(service.stateChecking()))
	service.Problem = service.problemOf(failure)
	service.Recipients = service.mapped(service.resolvingPending(service.byAddress(resolved)))
}

func (service *ComposerRecipientService) Remove(address string, role string) {
	service.Recipients = slices.DeleteFunc(slices.Clone(service.Recipients), service.matching(address, role))
}

func (service *ComposerRecipientService) RemoveLast(role string) {
	service.Recipients = service.withoutLast(service.InRole(role))
}

func (service *ComposerRecipientService) Choose(address string, role string, delivery string) {
	service.Recipients = service.mapped(service.choosing(service.keyOf(address, role), delivery))
}

func (service *ComposerRecipientService) Refresh() []RecipientChange {
	before := slices.Clone(service.Recipients)
	resolved, failure := service.resolveAny(service.addressesInState(service.stateReady()))
	service.Problem = service.problemOf(failure)
	service.Recipients = service.mapped(service.refreshing(service.byAddress(resolved)))
	return service.changesFrom(before)
}

func (service *ComposerRecipientService) Clear() {
	service.Recipients = []Recipient{}
	service.Problem = ""
}

func (service *ComposerRecipientService) InRole(role string) []Recipient {
	return slices.DeleteFunc(slices.Clone(service.Recipients), service.notInRole(role))
}

func (service *ComposerRecipientService) Find(address string) Recipient {
	return service.recipientAt(slices.IndexFunc(service.Recipients, service.withAddress(address)))
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

func (service *ComposerRecipientService) SenderAddress() string {
	return service.auth.Address()
}

func (service *ComposerRecipientService) addressesIn(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), service.isSeparator)
}

func (service *ComposerRecipientService) isSeparator(letter rune) bool {
	return strings.ContainsRune(" ,;\n\t", letter)
}

func (service *ComposerRecipientService) notYetAdded(addresses []string, role string) []string {
	known := service.keysOf(service.Recipients)
	return slices.DeleteFunc(slices.Compact(addresses), func(address string) bool {
		return slices.Contains(known, service.keyOf(address, role))
	})
}

func (service *ComposerRecipientService) chipsFor(addresses []string, role string) []Recipient {
	chips := []Recipient{}
	for _, address := range addresses {
		chips = append(chips, service.resolve.Pending(address, role, service.stateOfTyped(address)))
	}
	return chips
}

func (service *ComposerRecipientService) stateOfTyped(address string) string {
	states := map[bool]string{true: service.stateChecking(), false: service.stateInvalid()}
	return states[service.addressPattern().MatchString(address)]
}

func (service *ComposerRecipientService) resolveAny(addresses []string) ([]ResolvedRecipient, error) {
	if len(addresses) == 0 {
		return []ResolvedRecipient{}, nil
	}
	return service.resolve.Resolve(addresses)
}

func (service *ComposerRecipientService) addressesInState(state string) []string {
	addresses := []string{}
	for _, recipient := range service.Recipients {
		addresses = append(addresses, service.addressWhenIn(recipient, state)...)
	}
	return slices.Compact(addresses)
}

func (service *ComposerRecipientService) addressWhenIn(recipient Recipient, state string) []string {
	if recipient.State != state {
		return []string{}
	}
	return []string{recipient.Address}
}

func (service *ComposerRecipientService) resolvingPending(resolved map[string]ResolvedRecipient) func(Recipient) Recipient {
	return func(recipient Recipient) Recipient {
		return service.resolvedWhenPending(recipient, resolved)
	}
}

func (service *ComposerRecipientService) resolvedWhenPending(recipient Recipient, resolved map[string]ResolvedRecipient) Recipient {
	found, known := resolved[recipient.Address]
	if recipient.State != service.stateChecking() || !known {
		return recipient
	}
	return service.resolve.RecipientOf(found, recipient.Role, found.DefaultDelivery)
}

func (service *ComposerRecipientService) refreshing(resolved map[string]ResolvedRecipient) func(Recipient) Recipient {
	return func(recipient Recipient) Recipient {
		return service.refreshedWhenKnown(recipient, resolved)
	}
}

func (service *ComposerRecipientService) refreshedWhenKnown(recipient Recipient, resolved map[string]ResolvedRecipient) Recipient {
	found, known := resolved[recipient.Address]
	if !known {
		return recipient
	}
	return service.resolve.RecipientOf(found, recipient.Role, recipient.Delivery)
}

func (service *ComposerRecipientService) choosing(key string, delivery string) func(Recipient) Recipient {
	return func(recipient Recipient) Recipient {
		return service.resolve.Labelled(service.withDelivery(recipient, key, delivery))
	}
}

func (service *ComposerRecipientService) withDelivery(recipient Recipient, key string, delivery string) Recipient {
	if service.keyOf(recipient.Address, recipient.Role) != key || !slices.Contains(recipient.Allowed, delivery) {
		return recipient
	}
	recipient.Delivery = delivery
	return recipient
}

func (service *ComposerRecipientService) changesFrom(before []Recipient) []RecipientChange {
	changes := []RecipientChange{}
	for index, old := range before {
		changes = append(changes, service.changeWhenDifferent(old, service.Recipients[index])...)
	}
	return changes
}

func (service *ComposerRecipientService) changeWhenDifferent(old Recipient, fresh Recipient) []RecipientChange {
	if old.Kind == fresh.Kind && old.Delivery == fresh.Delivery {
		return []RecipientChange{}
	}
	sentence := fresh.DisplayName + " now gets: " + fresh.Label + "."
	return []RecipientChange{{Address: fresh.Address, DisplayName: fresh.DisplayName, Before: old.Label, After: fresh.Label, Sentence: sentence}}
}

func (service *ComposerRecipientService) byAddress(resolved []ResolvedRecipient) map[string]ResolvedRecipient {
	found := map[string]ResolvedRecipient{}
	for _, recipient := range resolved {
		found[strings.ToLower(recipient.Address)] = recipient
	}
	return found
}

func (service *ComposerRecipientService) withoutLast(inRole []Recipient) []Recipient {
	if len(inRole) == 0 {
		return service.Recipients
	}
	last := inRole[len(inRole)-1]
	return slices.DeleteFunc(slices.Clone(service.Recipients), service.matching(last.Address, last.Role))
}

func (service *ComposerRecipientService) recipientAt(index int) Recipient {
	if index < 0 {
		return Recipient{}
	}
	return service.Recipients[index]
}

func (service *ComposerRecipientService) choicesOf(recipients []Recipient) []RecipientChoice {
	choices := []RecipientChoice{}
	for _, recipient := range recipients {
		choices = append(choices, RecipientChoice{Address: recipient.Address, Role: recipient.Role, Delivery: recipient.Delivery, ExpectedKind: recipient.Kind})
	}
	return choices
}

func (service *ComposerRecipientService) keysOf(recipients []Recipient) []string {
	keys := []string{}
	for _, recipient := range recipients {
		keys = append(keys, service.keyOf(recipient.Address, recipient.Role))
	}
	return keys
}

func (service *ComposerRecipientService) mapped(change func(Recipient) Recipient) []Recipient {
	recipients := []Recipient{}
	for _, recipient := range service.Recipients {
		recipients = append(recipients, change(recipient))
	}
	return recipients
}

func (service *ComposerRecipientService) matching(address string, role string) func(Recipient) bool {
	key := service.keyOf(address, role)
	return func(recipient Recipient) bool {
		return service.keyOf(recipient.Address, recipient.Role) == key
	}
}

func (service *ComposerRecipientService) notInRole(role string) func(Recipient) bool {
	return func(recipient Recipient) bool {
		return recipient.Role != role
	}
}

func (service *ComposerRecipientService) withAddress(address string) func(Recipient) bool {
	return func(recipient Recipient) bool {
		return recipient.Address == address
	}
}

func (service *ComposerRecipientService) keyOf(address string, role string) string {
	return role + " " + strings.ToLower(address)
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

func (service *ComposerRecipientService) stateChecking() string {
	return "checking"
}

func (service *ComposerRecipientService) stateInvalid() string {
	return "invalid"
}

func (service *ComposerRecipientService) stateReady() string {
	return "ready"
}
