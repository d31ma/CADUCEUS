import (
	"regexp"
	"strings"
)

// ProviderContacts reaches the caller's CardDAV address books through the
// Go owner of external accounts: their contacts, and contact mutations.
type ProviderContacts struct {
	Accounts ExternalAccounts
}

// All answers the caller's provider contacts.
func (contacts ProviderContacts) All(token string) ([]any, error) {
	result, err := contacts.Accounts.Object("accounts.contacts", map[string]any{"token": token})
	listed, _ := RecordValues{}.List(result["contacts"])
	if listed == nil {
		listed = []any{}
	}
	return listed, err
}

// Available answers the caller's provider contacts, or none while provider
// sync cannot be reached: hosted contacts stay usable.
func (contacts ProviderContacts) Available(token string) []any {
	listed, err := contacts.All(token)
	if err != nil {
		return []any{}
	}
	return listed
}

// Find answers the caller's provider contact with an id, or nil.
func (contacts ProviderContacts) Find(token string, id string) (map[string]any, error) {
	listed, err := contacts.All(token)
	if err != nil {
		return nil, err
	}
	for _, candidate := range listed {
		if contact, _ := (RecordValues{}).Object(candidate); contact != nil && contact["id"] == id {
			return contact, nil
		}
	}
	return nil, nil
}

// Books answers the caller's provider address books.
func (contacts ProviderContacts) Books(token string) (any, error) {
	result, err := contacts.Accounts.Object("accounts.contact_books", map[string]any{"token": token})
	if err != nil {
		return nil, err
	}
	return RecordValues{}.Or(result["addressBooks"], []any{}), nil
}

// Mutate queues one contact mutation on an account.
func (contacts ProviderContacts) Mutate(token string, accountId any, input map[string]any) (map[string]any, error) {
	parameters := StoredRecords{}.Without(input)
	parameters["token"] = token
	if accountId != nil {
		parameters["id"] = accountId
	}
	return contacts.Accounts.Object("accounts.contact_mutate", parameters)
}

// Mutation answers one contact mutation of an account.
func (contacts ProviderContacts) Mutation(token string, accountId string, mutationId string) (map[string]any, error) {
	return contacts.Accounts.Object("accounts.contact_mutation", map[string]any{"token": token, "id": accountId, "mutationId": mutationId})
}

// Identifier reports whether value is a trimmed identifier with a prefix
// (ext_, book_), at most 256 units long and on one line.
func (ProviderContacts) Identifier(value any, prefix string) bool {
	text := Security{}.Trim(RecordValues{}.Text(value))
	return strings.HasPrefix(text, prefix) && (ContactsRules{}).Length(text) <= 256 && !regexp.MustCompile(`[\r\n\x00]`).MatchString(text)
}

// IdempotencyKey answers the client's key when it is usable, or a fresh one
// for the operation.
func (ProviderContacts) IdempotencyKey(value any, operation string) string {
	text := Security{}.Trim(RecordValues{}.Text(value))
	if text != "" && (ContactsRules{}).Length(text) <= 128 && !regexp.MustCompile(`[\r\n\x00]`).MatchString(text) {
		return text
	}
	return "contact-" + operation + "-" + Security{}.RandomUuid()
}
