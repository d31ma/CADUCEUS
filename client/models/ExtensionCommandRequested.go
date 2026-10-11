// ExtensionCommandRequested asks the extension bridge to send one command to
// a plugin's sandbox.
type ExtensionCommandRequested struct {
	ExtensionID string `json:"extensionId"`
	Nonce       string `json:"nonce"`
	RequestID   string `json:"requestId"`
	CommandID   string `json:"commandId"`
	Slot        string `json:"slot"`
}
