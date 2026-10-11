// ContactDraft is the body of a contact create or update. A CardDAV create
// also names the address book and an idempotency key.
type ContactDraft struct {
	Name           string         `json:"name"`
	GivenName      string         `json:"givenName"`
	FamilyName     string         `json:"familyName"`
	Company        string         `json:"company"`
	Title          string         `json:"title"`
	Birthday       string         `json:"birthday"`
	Notes          string         `json:"notes"`
	Nicknames      []string       `json:"nicknames"`
	Emails         []DraftValue   `json:"emails"`
	Phones         []DraftValue   `json:"phones"`
	Urls           []DraftValue   `json:"urls"`
	Addresses      []DraftAddress `json:"addresses"`
	Memberships    []string       `json:"groupIds"`
	Source         string         `json:"source,omitempty"`
	AccountID      string         `json:"accountId,omitempty"`
	AddressBookID  string         `json:"addressBookId,omitempty"`
	IdempotencyKey string         `json:"idempotencyKey,omitempty"`
}
