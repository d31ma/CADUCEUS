// InboxStreamPosition is how far one mail source of the inbox stream has
// been read: the receivedAt and id of its last streamed message.
type InboxStreamPosition struct {
	At string `json:"at"`
	Id string `json:"id"`
}
