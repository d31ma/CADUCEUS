type PasskeySignIn struct {
	ChallengeKey   string `json:"challengeKey"`
	CredentialJson string `json:"credentialJson"`
	DomainId       string `json:"domainId,omitempty"`
	KeepSignedIn   bool   `json:"keepSignedIn,omitempty"`
}
