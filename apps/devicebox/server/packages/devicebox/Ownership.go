package devicebox

import (
	"errors"

	"mesail.dev/packages/oidc"
)

type Ownership struct {
	directory DeviceDirectory
}

func NewOwnership(directory DeviceDirectory) Ownership {
	return Ownership{directory: directory}
}

func (ownership Ownership) ForPut(caller oidc.Caller, put CheckedPut) (map[string]string, error) {
	owners, err := ownership.owners(caller.DomainId, put.Devices())
	if err != nil {
		return nil, err
	}
	err = ownership.sender(owners, put.SenderDeviceId, caller.MemberId)
	if err != nil {
		return nil, err
	}
	return owners, ownership.receivers(owners, put.ToDeviceIds)
}

func (ownership Ownership) ForBox(caller oidc.Caller, deviceId string) error {
	err := ownership.boxId(deviceId)
	if err != nil {
		return err
	}
	owners, err := ownership.owners(caller.DomainId, []string{deviceId})
	if err != nil {
		return err
	}
	return ownership.boxOwner(owners[deviceId], caller.MemberId)
}

func (ownership Ownership) owners(domainId string, deviceIds []string) (map[string]string, error) {
	owners, err := ownership.directory.Owners(domainId, deviceIds)
	return owners, ownership.directoryError(err)
}

func (Ownership) directoryError(failure error) error {
	unavailable := &Unavailable{}
	if failure == nil || errors.As(failure, &unavailable) {
		return failure
	}
	return Unavailable{}.DirectoryDown(failure)
}

func (Ownership) sender(owners map[string]string, deviceId string, personId string) error {
	if owners[deviceId] != personId || personId == "" {
		return NotFound{}.SenderDevice()
	}
	return nil
}

func (Ownership) receivers(owners map[string]string, deviceIds []string) error {
	for _, deviceId := range deviceIds {
		if owners[deviceId] == "" {
			return NotFound{}.ReceivingDevice()
		}
	}
	return nil
}

func (Ownership) boxId(deviceId string) error {
	if !(Formats{}).DeviceId(deviceId) {
		return NotFound{}.Box()
	}
	return nil
}

func (Ownership) boxOwner(ownerId string, personId string) error {
	if ownerId == "" || ownerId != personId {
		return NotFound{}.Box()
	}
	return nil
}
