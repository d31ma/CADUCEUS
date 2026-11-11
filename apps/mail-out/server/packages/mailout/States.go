package mailout

type States struct{}

func (States) Queued() string   { return "queued" }
func (States) Sending() string  { return "sending" }
func (States) Sent() string     { return "sent" }
func (States) Deferred() string { return "deferred" }
func (States) Bounced() string  { return "bounced" }
func (States) Expired() string  { return "expired" }
func (States) Failed() string   { return "failed" }

func (states States) EventStatus(state string) string {
	if state == states.Expired() {
		return states.Failed()
	}
	return state
}
