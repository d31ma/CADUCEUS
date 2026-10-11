// ContactsShown is what the contacts page shows, retained so the editor, the
// group dialog and the vCard import and export hear the current selection.
type ContactsShown struct {
	Selected     Contact        `json:"selected"`
	Groups       []ContactGroup `json:"groups"`
	AddressBooks []AddressBook  `json:"addressBooks"`
	Checked      []string       `json:"checkedIds"`
	GroupFilter  string         `json:"groupFilter"`
}
