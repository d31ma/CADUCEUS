package devicebox

import "errors"

type MemoryDirectory struct {
	owners  map[string]string
	failing *bool
}

func NewMemoryDirectory() MemoryDirectory {
	return MemoryDirectory{owners: map[string]string{}, failing: new(bool)}
}

func (directory MemoryDirectory) Owns(domainId string, personId string, deviceIds ...string) {
	for _, deviceId := range deviceIds {
		directory.owners[directory.key(domainId, deviceId)] = personId
	}
}

func (directory MemoryDirectory) Fail(failing bool) {
	*directory.failing = failing
}

func (directory MemoryDirectory) Owners(domainId string, deviceIds []string) (map[string]string, error) {
	if *directory.failing {
		return nil, errors.New("the device list is down")
	}
	return directory.known(domainId, deviceIds), nil
}

func (directory MemoryDirectory) known(domainId string, deviceIds []string) map[string]string {
	owners := map[string]string{}
	for _, deviceId := range deviceIds {
		directory.copyOwner(owners, domainId, deviceId)
	}
	return owners
}

func (directory MemoryDirectory) copyOwner(owners map[string]string, domainId string, deviceId string) {
	personId, found := directory.owners[directory.key(domainId, deviceId)]
	if found {
		owners[deviceId] = personId
	}
}

func (MemoryDirectory) key(domainId string, deviceId string) string {
	return domainId + "/" + deviceId
}
