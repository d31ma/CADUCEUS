type RateLimited struct {
	ErrorCase
}

func (RateLimited) Locked() *RateLimited {
	return &RateLimited{ErrorCase{503, "rate_limited", "", "Too many sign-in attempts. Wait a few minutes."}}
}
