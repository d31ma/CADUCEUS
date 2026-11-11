type SignInRequest struct {
	Login       string `json:"login,omitempty"`
	Address     string `json:"address,omitempty"`
	Password    string `json:"password"`
	OneTimeCode string `json:"oneTimeCode,omitempty"`
}
