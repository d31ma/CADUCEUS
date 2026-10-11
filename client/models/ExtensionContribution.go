// ExtensionContribution is one command a plugin adds to a page slot, such as
// the app toolbar.
type ExtensionContribution struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Slot  string `json:"slot"`
}
