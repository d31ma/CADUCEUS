// SettingsActivity says that one settings section finished work, and
// whether the server refused a read that needs a valid session.
type SettingsActivity struct {
	Section string `json:"section"`
	Refused bool   `json:"refused"`
}
