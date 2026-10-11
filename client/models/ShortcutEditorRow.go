// ShortcutEditorRow is one editable command of the keyboard shortcut editor.
type ShortcutEditorRow struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	Binding      string `json:"binding"`
	InputBinding string `json:"inputBinding"`
	DefaultLabel string `json:"defaultLabel"`
}
