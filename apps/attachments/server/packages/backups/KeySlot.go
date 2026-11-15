package backups

type KeySlot struct {
	Kind         string `json:"kind"`
	Salt         string `json:"salt"`
	Rounds       int64  `json:"rounds"`
	CredentialId string `json:"credentialId"`
	WrappedKey   string `json:"wrappedKey"`
}
