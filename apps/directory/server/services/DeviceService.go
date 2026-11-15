import (
	"cmp"
	"maps"
	"slices"
	"strings"
	"time"
)

// @Service
type DeviceService struct {
	// @Needs
	devices *DeviceStorageService
	// @Needs
	backupKeys *BackupKeyStorageService
	// @Needs
	people *PeopleStorageService
}

func (service *DeviceService) Add(personId string, add DeviceAdd) (Device, error) {
	add = DeviceInput{}.Cleaned(add)
	existing, failure := service.devices.ByPerson(personId)
	failure = cmp.Or(DeviceInput{}.Check(add), failure)
	failure = Steps{}.Unless(failure, func() error { return service.unused(add.DeviceId) })
	failure = Steps{}.Unless(failure, func() error { return service.trusted(personId, existing, add) })
	failure = Steps{}.Unless(failure, func() error { return service.keepBackupKey(personId, add.BackupKey) })
	stored, failure := service.storeUnless(failure, service.document(personId, add))
	return service.view(stored, personId), failure
}

func (service *DeviceService) List(domainId string, callerId string, personIdsText string) ([]Device, error) {
	personIds, invalid := DeviceInput{}.PersonIds(personIdsText)
	devices, failure := service.listUnless(invalid, domainId, personIds)
	return service.views(devices, callerId), cmp.Or(invalid, failure)
}

func (service *DeviceService) Remove(personId string, deviceId string) error {
	device, found, failure := service.devices.ByDeviceId(deviceId)
	failure = cmp.Or(failure, service.ownDevice(found, device.PersonId, personId))
	return Steps{}.Unless(failure, func() error { return service.devices.Delete(device) })
}

func (service *DeviceService) RemoveAll(personId string) error {
	devices, failure := service.devices.ByPerson(personId)
	return Steps{}.Unless(failure, func() error { return service.deleteEach(devices) })
}

func (service *DeviceService) Forget(personId string) error {
	failure := service.RemoveAll(personId)
	return Steps{}.Unless(failure, func() error { return service.backupKeys.Delete(personId) })
}

func (service *DeviceService) Owners(domainId string, deviceIds []string) (DeviceOwners, error) {
	ids, invalid := DeviceInput{}.DeviceIds(deviceIds)
	devices, failure := service.foundUnless(invalid, ids)
	members, memberFailure := service.members(strings.ToLower(strings.TrimSpace(domainId)), devices)
	return DeviceOwners{Owners: service.owners(devices, members)}, cmp.Or(invalid, failure, memberFailure)
}

func (service *DeviceService) unused(deviceId string) error {
	_, found, failure := service.devices.ByDeviceId(deviceId)
	return cmp.Or(failure, service.duplicateWhen(found))
}

func (service *DeviceService) duplicateWhen(found bool) error {
	if found {
		return Duplicate{}.DeviceInUse()
	}
	return nil
}

func (service *DeviceService) trusted(personId string, existing []DeviceDocument, add DeviceAdd) error {
	if len(existing) == 0 {
		return nil
	}
	return service.signed(personId, existing, add)
}

func (service *DeviceService) signed(personId string, existing []DeviceDocument, add DeviceAdd) error {
	signerKey, failure := service.signerKey(personId, existing, add.SignerDeviceId)
	return cmp.Or(failure, service.signatureRefusal(DeviceInput{}.SignedBy(signerKey, add)), service.lockedBackupKey(add))
}

func (service *DeviceService) signerKey(personId string, existing []DeviceDocument, signerId string) (string, error) {
	if signerId == service.backupSigner() {
		return service.backupKey(personId)
	}
	return service.identityKeys(existing)[signerId], nil
}

func (service *DeviceService) backupKey(personId string) (string, error) {
	key, failure := service.backupKeys.ByPerson(personId)
	return key.BackupKey, failure
}

func (service *DeviceService) identityKeys(devices []DeviceDocument) map[string]string {
	keys := map[string]string{}
	for _, device := range devices {
		keys[device.DeviceId] = device.IdentityKey
	}
	return keys
}

func (service *DeviceService) signatureRefusal(verified bool) error {
	if verified {
		return nil
	}
	return Forbidden{}.DeviceNotSigned()
}

func (service *DeviceService) lockedBackupKey(add DeviceAdd) error {
	if add.SignerDeviceId == service.backupSigner() && add.BackupKey != "" {
		return InvalidRequest{}.BackupKeyLocked()
	}
	return nil
}

func (service *DeviceService) keepBackupKey(personId string, backupKey string) error {
	if backupKey == "" {
		return nil
	}
	return service.backupKeys.Store(personId, backupKey)
}

func (service *DeviceService) storeUnless(failure error, device DeviceDocument) (DeviceDocument, error) {
	if failure != nil {
		return DeviceDocument{}, failure
	}
	return service.devices.Insert(device)
}

func (service *DeviceService) document(personId string, add DeviceAdd) DeviceDocument {
	return DeviceDocument{DeviceId: add.DeviceId, PersonId: personId, Name: add.Name, IdentityKey: add.IdentityKey, AgreementKey: add.AgreementKey, AddedAt: time.Now().UTC().Format(time.RFC3339Nano)}
}

func (service *DeviceService) listUnless(invalid error, domainId string, personIds []string) ([]DeviceDocument, error) {
	if invalid != nil {
		return []DeviceDocument{}, invalid
	}
	return service.ofMembers(domainId, personIds)
}

func (service *DeviceService) ofMembers(domainId string, personIds []string) ([]DeviceDocument, error) {
	found := []DeviceDocument{}
	var failure error
	for _, personId := range personIds {
		devices, personFailure := service.ofMember(domainId, personId)
		found = append(found, devices...)
		failure = cmp.Or(failure, personFailure)
	}
	return found, failure
}

func (service *DeviceService) ofMember(domainId string, personId string) ([]DeviceDocument, error) {
	member, failure := service.member(domainId, personId)
	devices, deviceFailure := service.devicesWhen(member, personId)
	return devices, cmp.Or(failure, deviceFailure)
}

func (service *DeviceService) member(domainId string, personId string) (bool, error) {
	person, _, failure := service.people.ByPersonId(personId)
	return service.activeMember(person, domainId), failure
}

func (service *DeviceService) activeMember(person PersonDocument, domainId string) bool {
	return person.State == service.active() && (PersonRules{}).Member(person, domainId)
}

func (service *DeviceService) devicesWhen(member bool, personId string) ([]DeviceDocument, error) {
	if !member {
		return []DeviceDocument{}, nil
	}
	return service.devices.ByPerson(personId)
}

func (service *DeviceService) views(devices []DeviceDocument, callerId string) []Device {
	shown := []Device{}
	for _, device := range devices {
		shown = append(shown, service.view(device, callerId))
	}
	return shown
}

func (service *DeviceService) view(device DeviceDocument, callerId string) Device {
	return Device{DeviceId: device.DeviceId, PersonId: device.PersonId, Name: service.nameFor(device, callerId), IdentityKey: device.IdentityKey, AgreementKey: device.AgreementKey, AddedAt: device.AddedAt}
}

func (service *DeviceService) nameFor(device DeviceDocument, callerId string) string {
	if device.PersonId != callerId {
		return ""
	}
	return device.Name
}

func (service *DeviceService) ownDevice(found bool, ownerId string, personId string) error {
	if !found || ownerId != personId {
		return NotFound{}.Device()
	}
	return nil
}

func (service *DeviceService) deleteEach(devices []DeviceDocument) error {
	var failure error
	for _, device := range devices {
		failure = cmp.Or(failure, service.devices.Delete(device))
	}
	return failure
}

func (service *DeviceService) foundUnless(invalid error, deviceIds []string) ([]DeviceDocument, error) {
	if invalid != nil {
		return []DeviceDocument{}, nil
	}
	return service.found(deviceIds)
}

func (service *DeviceService) found(deviceIds []string) ([]DeviceDocument, error) {
	found := []DeviceDocument{}
	var failure error
	for _, deviceId := range deviceIds {
		devices, deviceFailure := service.devices.Matching(deviceId)
		found = append(found, devices...)
		failure = cmp.Or(failure, deviceFailure)
	}
	return found, failure
}

func (service *DeviceService) members(domainId string, devices []DeviceDocument) (map[string]bool, error) {
	members := map[string]bool{}
	var failure error
	for _, personId := range service.ownerIds(devices) {
		member, personFailure := service.member(domainId, personId)
		members[personId] = member
		failure = cmp.Or(failure, personFailure)
	}
	return members, failure
}

func (service *DeviceService) ownerIds(devices []DeviceDocument) []string {
	seen := map[string]bool{}
	for _, device := range devices {
		seen[device.PersonId] = true
	}
	return slices.Sorted(maps.Keys(seen))
}

func (service *DeviceService) owners(devices []DeviceDocument, members map[string]bool) map[string]string {
	owners := map[string]string{}
	for _, device := range devices {
		service.addOwner(owners, device, members[device.PersonId])
	}
	return owners
}

func (service *DeviceService) addOwner(owners map[string]string, device DeviceDocument, member bool) {
	if member {
		owners[device.DeviceId] = device.PersonId
	}
}

func (service *DeviceService) backupSigner() string {
	return "backup"
}

func (service *DeviceService) active() string {
	return "active"
}
