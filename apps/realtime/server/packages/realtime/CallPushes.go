package realtime

type CallPushes interface {
	Send(domainId string, push CallPush) error
}
