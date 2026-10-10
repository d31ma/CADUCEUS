// ContactFilters narrow a contact listing: a text search, a source
// (manual, mail, vcard or carddav) and a group id; "" leaves one open.
type ContactFilters struct {
	Query   string
	Source  string
	GroupId string
}
