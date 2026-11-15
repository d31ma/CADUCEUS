type SignInRequest struct {
	Login        string `json:"login"`
	Password     string `json:"password"`
	OneTimeCode  string `json:"oneTimeCode,omitempty"`
	KeepSignedIn bool   `json:"keepSignedIn,omitempty"`
}
