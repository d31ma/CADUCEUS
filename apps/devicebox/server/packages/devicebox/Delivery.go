package devicebox

type Delivery struct {
	EnvelopeId string
	Kind       string
	DeviceIds  []string
	Owners     map[string]string
}
