type SessionTokenRequest struct {
	DomainId     string `json:"domainId"`
	Code         string `json:"code"`
	CodeVerifier string `json:"codeVerifier"`
	RedirectUri  string `json:"redirectUri"`
}
