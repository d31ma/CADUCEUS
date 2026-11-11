type ClientConfig struct {
	Issuer      string   `json:"issuer"`
	ClientId    string   `json:"clientId"`
	Scopes      []string `json:"scopes"`
	ApiBase     string   `json:"apiBase"`
	RedirectUri string   `json:"redirectUri"`
}
