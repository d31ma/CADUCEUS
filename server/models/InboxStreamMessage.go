// InboxStreamMessage is one message of the live new-mail socket: an event
// (connected, new-email, ping, reconnect, error or unauthorized), its data,
// and the cursor a reconnect resumes from (null when the event moves none).
type InboxStreamMessage struct {
	Event  string         `json:"event"`
	Data   map[string]any `json:"data"`
	Cursor *string        `json:"cursor"`
}
