package realtime

type ClientMessage struct {
	Type              string   `json:"type"`
	Token             string   `json:"token,omitempty"`
	Ticket            string   `json:"ticket,omitempty"`
	ConversationId    string   `json:"conversationId,omitempty"`
	State             string   `json:"state,omitempty"`
	DoNotDisturbUntil string   `json:"doNotDisturbUntil,omitempty"`
	DeviceId          string   `json:"deviceId,omitempty"`
	RequestId         string   `json:"requestId,omitempty"`
	ToDeviceIds       []string `json:"toDeviceIds,omitempty"`
	Kind              string   `json:"kind,omitempty"`
	Ciphertext        string   `json:"ciphertext,omitempty"`
	PairingId         string   `json:"pairingId,omitempty"`
	CallId            string   `json:"callId,omitempty"`
	Media             string   `json:"media,omitempty"`
	Group             bool     `json:"group,omitempty"`
	ToPersonIds       []string `json:"toPersonIds,omitempty"`
	Invite            string   `json:"invite,omitempty"`
	Missed            []string `json:"missed,omitempty"`
}
