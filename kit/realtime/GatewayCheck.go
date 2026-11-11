package realtime

type GatewayCheck struct {
	Secret string
}

func (check GatewayCheck) Allowed(header string) bool {
	return false
}
