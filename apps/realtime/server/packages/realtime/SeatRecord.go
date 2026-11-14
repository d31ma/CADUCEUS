package realtime

type SeatRecord struct {
	ID       string `json:"id,omitempty"`
	CallId   string `json:"callId"`
	DeviceId string `json:"deviceId"`
	PersonId string `json:"personId"`
	JoinedAt string `json:"joinedAt"`
}
