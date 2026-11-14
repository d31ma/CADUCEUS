package realtime

type CallPushLog interface {
	PushFailed(domainId string, kind string, failure error)
}
