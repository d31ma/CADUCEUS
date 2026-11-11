// @HTTPClient("/api/v1/realtime")
type RealtimeClient interface {
	// @SocketRequest("/socket")
	Receive(event RealtimeEvent)
}
