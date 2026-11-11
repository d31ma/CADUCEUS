type GatewayUnavailable struct {
	Operation string
}

func (failure *GatewayUnavailable) Error() string {
	return "the connection gateway failed: " + failure.Operation
}
