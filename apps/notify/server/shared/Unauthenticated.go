type Unauthenticated struct {
	ErrorCase
}

func (Unauthenticated) TokenRefused() *Unauthenticated {
	return &Unauthenticated{ErrorCase{4501, "unauthenticated", "", "Sign in again."}}
}
