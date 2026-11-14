package devicebox

type DeviceDirectory interface {
	Owners(domainId string, deviceIds []string) (map[string]string, error)
}
