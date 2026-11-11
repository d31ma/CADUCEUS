package transport

type TicketMessage struct {
	Type   string `json:"type"`
	Ticket string `json:"ticket"`
}
