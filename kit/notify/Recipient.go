package notify

type Recipient struct {
	MemberId string `json:"memberId"`
	Kind     string `json:"kind"`
}
