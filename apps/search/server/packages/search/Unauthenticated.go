package search

type Unauthenticated struct{}

func (Unauthenticated) Error() string {
	return "the access token was refused"
}
