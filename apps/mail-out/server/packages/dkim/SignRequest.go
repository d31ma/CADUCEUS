package dkim

import "time"

type SignRequest struct {
	Message []byte
	Key     Key
	Time    time.Time
	Headers []string
}
