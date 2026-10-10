// RateLimitService limits requests per key with windows stored in FYLO, which
// every self-hosted application process shares.
// @Service
type RateLimitService struct {
	// @Needs
	fyloHelper *FyloHelper
}

// Check counts one request for the key (its parts are joined with ":")
// against maxRequests per window of windowMilliseconds.
func (service *RateLimitService) Check(key []string, maxRequests int, windowMilliseconds int64) (RateLimitResult, error) {
	return service.limiter().Checked(key, maxRequests, windowMilliseconds)
}

func (service *RateLimitService) limiter() RateLimiter {
	return RateLimiter{Store: FyloCollection{Database: service.fyloHelper.Database(), Name: FyloCollections{}.Named().RateLimits}}
}
