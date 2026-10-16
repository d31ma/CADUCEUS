// The editor body for a send, or Accepted false when the person kept an empty
// template blank.
type SendBodyReady struct {
	QuoteProof *QuoteProof `json:"quoteProof,omitempty"`
	HTML       string      `json:"html"`
	Text       string      `json:"text"`
	Accepted   bool        `json:"accepted"`
	TooLarge   bool        `json:"tooLarge"`
}
