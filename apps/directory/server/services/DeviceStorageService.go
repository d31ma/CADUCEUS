// @Service
type DeviceStorageService struct {
	// @Needs
	documents *DocumentsService
	// @Needs
	names *CollectionNamesService
}

func (storage *DeviceStorageService) ByDeviceId(deviceId string) (DeviceDocument, bool, error) {
	device := DeviceDocument{}
	found, failure := storage.documents.FindOne(storage.names.Devices(), map[string]any{storage.deviceField(): deviceId}, &device)
	return device, found, failure
}

func (storage *DeviceStorageService) Matching(deviceId string) ([]DeviceDocument, error) {
	return storage.where(storage.deviceField(), deviceId)
}

func (storage *DeviceStorageService) ByPerson(personId string) ([]DeviceDocument, error) {
	return storage.where(storage.personField(), personId)
}

func (storage *DeviceStorageService) Insert(device DeviceDocument) (DeviceDocument, error) {
	id, failure := storage.documents.Insert(storage.names.Devices(), device)
	device.Id = id
	return device, failure
}

func (storage *DeviceStorageService) Delete(device DeviceDocument) error {
	return storage.documents.Delete(storage.names.Devices(), device.Id)
}

func (storage *DeviceStorageService) where(field string, value string) ([]DeviceDocument, error) {
	devices := []DeviceDocument{}
	failure := storage.documents.All(storage.names.Devices(), map[string]any{field: value}, &devices)
	return devices, failure
}

func (storage *DeviceStorageService) deviceField() string {
	return "deviceId"
}

func (storage *DeviceStorageService) personField() string {
	return "personId"
}
