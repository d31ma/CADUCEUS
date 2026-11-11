package realtime

import "net/http"

type HttpConnectionGateway struct {
	BaseUrl string
	Secret  string
	Client  *http.Client
}

func (gateway HttpConnectionGateway) Push(connectionId string, event Event) error {
	return nil
}

func (gateway HttpConnectionGateway) Close(connectionId string, code int) error {
	return nil
}
