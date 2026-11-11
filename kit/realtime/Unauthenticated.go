package realtime

type Unauthenticated struct{}

func (Unauthenticated) Error() string {
	return "the access token is missing, expired or not valid"
}
