// FailureUtils reads the Tachyon error code of a failed browser request,
// such as HTTP_STATUS for an error answer or HTTP_NETWORK when the request
// never reached the server.
type FailureUtils struct{}

func (FailureUtils) Code(failure error) string {
	coded, _ := failure.(interface{ BehaviorCode() string })
	return FailureUtils{}.code(coded)
}

func (FailureUtils) code(coded interface{ BehaviorCode() string }) string {
	if coded == nil {
		return ""
	}
	return coded.BehaviorCode()
}

func (FailureUtils) OfflineCode() string { return "HTTP_NETWORK" }
