// WebPushContent is the public new-mail notification payload the service
// worker shows, with fields in the order the former payload wrote them.
type WebPushContent struct {
	Title   string          `json:"title"`
	Body    any             `json:"body"`
	Data    WebPushData     `json:"data"`
	Tag     string          `json:"tag"`
	Silent  bool            `json:"silent"`
	Actions []WebPushAction `json:"actions,omitempty"`
}
