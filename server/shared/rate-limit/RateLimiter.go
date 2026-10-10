import "math"

// RateLimiter counts requests per key in fixed windows stored in FYLO, so
// every application process shares one count. Times are whole seconds.
type RateLimiter struct {
	Store CollectionStore
}

// Checked counts one request against a limit of maximumRequests per window
// and answers the result.
func (limiter RateLimiter) Checked(key []string, maximumRequests int, windowMilliseconds int64) (RateLimitResult, error) {
	windowSeconds := int64(math.Ceil(float64(windowMilliseconds) / 1000))
	hash := RateLimitRules{}.HashKey(key)
	now := EpochTime{}.Now() / 1000
	documents, err := limiter.Store.Equal("key", hash)
	if err != nil {
		return RateLimitResult{}, err
	}
	count, resetAt := 1, now+windowSeconds
	if len(documents) == 0 {
		_, err = limiter.Store.Put(map[string]any{"key": hash, "count": 1, "resetAt": resetAt})
		return RateLimitRules{}.Result(count, maximumRequests, resetAt, now), err
	}
	record := StoredRecords{}.Decoded(documents[0].Data)
	storedReset, _ := RecordValues{}.Number(record["resetAt"])
	if storedReset <= float64(now) {
		err = limiter.Store.Patch(documents[0].Id, map[string]any{"count": 1, "resetAt": resetAt})
		return RateLimitRules{}.Result(count, maximumRequests, resetAt, now), err
	}
	storedCount, _ := RecordValues{}.Number(record["count"])
	count, resetAt = int(storedCount)+1, int64(storedReset)
	err = limiter.Store.Patch(documents[0].Id, map[string]any{"count": count})
	return RateLimitRules{}.Result(count, maximumRequests, resetAt, now), err
}
