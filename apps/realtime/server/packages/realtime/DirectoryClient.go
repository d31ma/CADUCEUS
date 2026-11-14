package realtime

type DirectoryClient struct {
	Call ServiceCall
}

func (client DirectoryClient) Owners(domainId string, deviceIds []string) (map[string]string, error) {
	answer := DeviceOwners{}
	err := client.Call.Post(domainId, "/api/v1/directory/devices/owners", DeviceOwnersQuery{DomainId: domainId, DeviceIds: deviceIds}, &answer)
	return client.owners(answer), err
}

func (DirectoryClient) owners(answer DeviceOwners) map[string]string {
	if answer.Owners == nil {
		return map[string]string{}
	}
	return answer.Owners
}
