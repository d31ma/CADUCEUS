type BackupKeySlot struct {
	Kind         string `json:"kind"`
	Salt         string `json:"salt"`
	Rounds       int64  `json:"rounds"`
	CredentialId string `json:"credentialId,omitempty"`
	WrappedKey   string `json:"wrappedKey"`
}
