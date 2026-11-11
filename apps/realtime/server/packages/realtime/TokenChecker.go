package realtime

import "time"

type TokenChecker interface {
	Grant(token string, now time.Time) (Grant, error)
}
