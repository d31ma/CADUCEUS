// Contact is one entry of GET /contacts: a personal, found-in-mail or
// imported contact, or a contact of a synced CardDAV address book. A write
// to a CardDAV contact answers its syncState and the mutation to follow.
type Contact struct {
	ID            string           `json:"id"`
	Name          string           `json:"name"`
	GivenName     string           `json:"givenName"`
	FamilyName    string           `json:"familyName"`
	Company       string           `json:"company"`
	Title         string           `json:"title"`
	Birthday      string           `json:"birthday"`
	Notes         string           `json:"notes"`
	Nicknames     []string         `json:"nicknames"`
	Emails        []ContactValue   `json:"emails"`
	Phones        []ContactValue   `json:"phones"`
	Urls          []ContactValue   `json:"urls"`
	Addresses     []ContactAddress `json:"addresses"`
	Memberships   []string         `json:"groupIds"`
	Source        string           `json:"source"`
	ReadOnly      bool             `json:"readOnly"`
	AccountID     string           `json:"accountId"`
	AddressBookID string           `json:"addressBookId"`
	SyncState     string           `json:"syncState"`
	MutationID    string           `json:"mutationId"`
}
