package realtime

type Gone struct {
	ConnectionId string
}

func (Gone) Error() string {
	return "the connection is gone"
}
