type RateLimited struct{}

func (failure *RateLimited) Error() string {
	return "too many sign-in attempts; wait a few minutes"
}
