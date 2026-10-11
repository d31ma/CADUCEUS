// ExtensionReply is a sandbox message read into its fields: "ready", a
// "result" (JSON text, filtered by the plugin's permissions) or an "error".
type ExtensionReply struct {
	ExtensionID string `json:"extensionId"`
	Nonce       string `json:"nonce"`
	Kind        string `json:"kind"`
	RequestID   string `json:"requestId"`
	Result      string `json:"result"`
	Error       string `json:"error"`
}
