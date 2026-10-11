// ContactForm is the contact editor's state. Nicknames is the comma-separated
// text of the field.
type ContactForm struct {
	Name        string            `json:"name"`
	GivenName   string            `json:"givenName"`
	FamilyName  string            `json:"familyName"`
	Company     string            `json:"company"`
	Title       string            `json:"title"`
	Birthday    string            `json:"birthday"`
	Notes       string            `json:"notes"`
	Nicknames   string            `json:"nicknames"`
	Emails      []EditableValue   `json:"emails"`
	Phones      []EditableValue   `json:"phones"`
	Urls        []EditableValue   `json:"urls"`
	Addresses   []EditableAddress `json:"addresses"`
	Memberships []string          `json:"groupIds"`
}
