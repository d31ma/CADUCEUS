// ShortcutEditorGroup is one group of commands in the keyboard shortcut
// editor of the settings page.
type ShortcutEditorGroup struct {
	Label string              `json:"label"`
	Rows  []ShortcutEditorRow `json:"rows"`
}
