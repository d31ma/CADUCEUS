package backend

import "time"

type Attempt struct {
	Path    string
	Payload any
	Delay   time.Duration
}
