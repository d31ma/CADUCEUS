package realtime

type CallFrame struct {
	Type         string `json:"type"`
	RequestId    string `json:"requestId,omitempty"`
	CallId       string `json:"callId"`
	JoinToken    string `json:"joinToken,omitempty"`
	FromPersonId string `json:"fromPersonId,omitempty"`
	Invite       string `json:"invite,omitempty"`
}
