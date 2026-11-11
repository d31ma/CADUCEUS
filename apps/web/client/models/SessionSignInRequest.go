type SessionSignInRequest struct {
	Address     string `json:"address"`
	Password    string `json:"password"`
	OneTimeCode string `json:"oneTimeCode,omitempty"`
}
