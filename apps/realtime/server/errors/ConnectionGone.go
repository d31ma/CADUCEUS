type ConnectionGone struct {
	ConnectionId string
}

func (failure *ConnectionGone) Error() string {
	return "the connection is gone"
}
