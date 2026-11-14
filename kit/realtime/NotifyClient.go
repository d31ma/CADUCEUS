package realtime

type NotifyClient struct {
	Call ServiceCall
}

func (client NotifyClient) Send(domainId string, push CallPush) error {
	return client.Call.Post(domainId, "/api/v1/notify/call-pushes", push, nil)
}
