// ExtensionAnswered is what a plugin sandbox said: "ready", a "result" (as
// JSON text, filtered by the plugin's permissions in Go) or an "error".
type ExtensionAnswered struct {
	ExtensionID string `json:"extensionId"`
	Nonce       string `json:"nonce"`
	Kind        string `json:"kind"`
	RequestID   string `json:"requestId"`
	Result      string `json:"result"`
	Error       string `json:"error"`
}
