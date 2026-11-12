type Unavailable struct {
	ErrorCase
}

func (Unavailable) SignIn() *Unavailable {
	return &Unavailable{ErrorCase{504, "unavailable", "", "Sign-in is not available now. Try again later."}}
}

func (Unavailable) ServiceTokens() *Unavailable {
	return &Unavailable{ErrorCase{519, "unavailable", "", "Service tokens are not available now. Try again later."}}
}

func (Unavailable) SigningKeys() *Unavailable {
	return &Unavailable{ErrorCase{520, "unavailable", "", "The signing keys are not available now. Try again later."}}
}
