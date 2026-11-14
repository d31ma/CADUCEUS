package outbox

import "mesail.dev/packages/events"

type DeadLetter struct {
	Envelope   events.Envelope `json:"envelope"`
	Subscriber string          `json:"subscriber"`
	Attempts   int             `json:"attempts"`
	LastError  string          `json:"lastError"`
	DeadAt     string          `json:"deadAt"`
}
