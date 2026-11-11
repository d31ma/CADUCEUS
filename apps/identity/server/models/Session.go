type Session struct {
	Me                 Me             `json:"me"`
	Domains            []DomainChoice `json:"domains"`
	AccessToken        string         `json:"accessToken"`
	ExpiresIn          int64          `json:"expiresIn"`
	SessionKey         string         `json:"sessionKey"`
	MustChangePassword bool           `json:"mustChangePassword"`
}
