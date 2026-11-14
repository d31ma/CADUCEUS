package realtime

type Unauthenticated struct {
	ErrorNumber int
}

func (Unauthenticated) TokenRefused() Unauthenticated {
	return Unauthenticated{ErrorNumber: 5011}
}

func (Unauthenticated) MemberTokenRefused() Unauthenticated {
	return Unauthenticated{ErrorNumber: 5012}
}

func (Unauthenticated) TicketRefused() Unauthenticated {
	return Unauthenticated{ErrorNumber: 5013}
}

func (Unauthenticated) NotSignedIn() Unauthenticated {
	return Unauthenticated{ErrorNumber: 5014}
}

func (Unauthenticated) SignInEnded() Unauthenticated {
	return Unauthenticated{ErrorNumber: 5015}
}

func (Unauthenticated) OtherMember() Unauthenticated {
	return Unauthenticated{ErrorNumber: 5016}
}

func (Unauthenticated) DeviceOfAnotherPerson() Unauthenticated {
	return Unauthenticated{ErrorNumber: 5027}
}

func (Unauthenticated) OtherDevice() Unauthenticated {
	return Unauthenticated{ErrorNumber: 5046}
}

func (Unauthenticated) DeviceRemoved() Unauthenticated {
	return Unauthenticated{ErrorNumber: 5047}
}

func (Unauthenticated) Error() string {
	return "the access token is missing, expired or not valid"
}

func (failure Unauthenticated) Number() int {
	return failure.ErrorNumber
}
