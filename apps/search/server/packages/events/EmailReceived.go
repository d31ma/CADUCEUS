package events

type EmailReceived struct {
	InboundId   string       `json:"inboundId"`
	Rcpts       []string     `json:"rcpts"`
	RawBlobKey  string       `json:"rawBlobKey"`
	Headers     EmailHeaders `json:"headers"`
	AuthResults AuthResults  `json:"authResults"`
}
