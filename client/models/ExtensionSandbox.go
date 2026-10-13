// ExtensionSandbox is what the sandbox host needs to start one enabled
// plugin: its id, its session nonce and its source.
type ExtensionSandbox struct {
	ID     string `json:"id"`
	Nonce  string `json:"nonce"`
	Source string `json:"source"`
}
