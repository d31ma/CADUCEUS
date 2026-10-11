// ExtensionSandbox is one enabled plugin's opaque iframe: its title and the
// srcdoc that starts the plugin in a Blob worker.
type ExtensionSandbox struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Document string `json:"document"`
}
