type EmailReceivedPayload struct {
	InboundId   string         `json:"inboundId"`
	Rcpts       []string       `json:"rcpts"`
	RawBlobKey  string         `json:"rawBlobKey"`
	Headers     InboundHeaders `json:"headers"`
	AuthResults AuthResults    `json:"authResults"`
}
