type ProtectionOptions struct {
	AllowRemote   bool              `json:"allowRemote"`
	InlineSources map[string]string `json:"inlineSources"`
	RemoteSources map[string]string `json:"remoteSources"`
	ShowQuoted    bool              `json:"showQuoted"`
	ColorMode     string            `json:"colorMode"`
}
