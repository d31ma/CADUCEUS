package devicebox

type EnvelopePage struct {
	Envelopes []WaitingEnvelope `json:"envelopes"`
	Next      string            `json:"next"`
}
