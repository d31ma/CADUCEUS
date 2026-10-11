// The editor replaced its whole body (signature, template, translation).
// ComposeService rebuilds the editor document from it.
type EditorReplaced struct {
	HTML   string `json:"html"`
	Reason string `json:"reason"`
}
