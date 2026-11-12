package login

import "net/http"

type DirectoryReply struct {
	Status      int
	Body        []byte
	Failed      error
	Unreachable bool
}

func (reply DirectoryReply) Retryable() bool {
	return reply.Unreachable || reply.Status == http.StatusBadGateway || reply.Status == http.StatusServiceUnavailable
}
