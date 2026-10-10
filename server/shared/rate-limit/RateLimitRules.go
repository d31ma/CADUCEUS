import "strings"

// RateLimitRules names rate limit windows by the hash of their key and
// formats check results.
type RateLimitRules struct{}

// HashKey answers the stored key of a limit: the SHA-256 hex of its parts
// joined with ":".
func (RateLimitRules) HashKey(key []string) string {
	return Security{}.Sha256Hex(strings.Join(key, ":"))
}

// Result answers the check result of the count-th request of a window.
func (RateLimitRules) Result(count int, maximumRequests int, resetAt int64, now int64) RateLimitResult {
	return RateLimitResult{
		Allowed:           count <= maximumRequests,
		Limit:             maximumRequests,
		Remaining:         max(0, maximumRequests-count),
		ResetAt:           resetAt,
		RetryAfterSeconds: resetAt - now,
	}
}
