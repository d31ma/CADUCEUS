// RateLimitResult is the answer of one rate limit check.
type RateLimitResult struct {
	Allowed           bool  `json:"allowed"`
	Limit             int   `json:"limit"`
	Remaining         int   `json:"remaining"`
	ResetAt           int64 `json:"resetAt"`
	RetryAfterSeconds int64 `json:"retryAfterSeconds"`
}
