type Unauthenticated struct {
	ErrorCase
}

func (Unauthenticated) TokenRefused() *Unauthenticated {
	return &Unauthenticated{ErrorCase{1001, "unauthenticated", "", "the access token was refused"}}
}
