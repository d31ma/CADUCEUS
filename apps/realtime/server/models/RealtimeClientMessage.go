type RealtimeClientMessage struct {
	Type              string `json:"type"`
	Token             string `json:"token,omitempty"`
	Ticket            string `json:"ticket,omitempty"`
	ConversationId    string `json:"conversationId,omitempty"`
	State             string `json:"state,omitempty"`
	DoNotDisturbUntil string `json:"doNotDisturbUntil,omitempty"`
}
