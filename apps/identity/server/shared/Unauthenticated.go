type Unauthenticated struct {
	ErrorCase
}

func (Unauthenticated) SignInFailed() *Unauthenticated {
	return &Unauthenticated{ErrorCase{501, "unauthenticated", "", "Sign-in failed. Check your details and try again."}}
}

func (Unauthenticated) TokenRefused() *Unauthenticated {
	return &Unauthenticated{ErrorCase{502, "unauthenticated", "", "Your sign-in has ended. Sign in again."}}
}

func (Unauthenticated) ClientRefused() *Unauthenticated {
	return &Unauthenticated{ErrorCase{518, "unauthenticated", "", "The client id or client secret is not right."}}
}
