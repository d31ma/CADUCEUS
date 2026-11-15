import "mesail.dev/packages/backups"

// @Service
type BackupViewService struct {
}

func (service *BackupViewService) Backup(backup backups.Backup) Backup {
	return Backup{
		Version: backup.Version, Size: backup.Size, Sha256: backup.Sha256, KeySlots: service.wireSlots(backup.KeySlots),
		UpdatedAt: backup.UpdatedAt, UpdatedByDeviceId: backup.UpdatedByDeviceId,
	}
}

func (service *BackupViewService) Start(request BackupStart) backups.StartRequest {
	return backups.StartRequest{
		ExpectedVersion: request.ExpectedVersion, Size: request.Size, Sha256: request.Sha256,
		KeySlots: service.storedSlots(request.KeySlots), DeviceId: request.DeviceId,
	}
}

func (service *BackupViewService) wireSlots(slots []backups.KeySlot) []BackupKeySlot {
	wire := []BackupKeySlot{}
	for _, slot := range slots {
		wire = append(wire, BackupKeySlot(slot))
	}
	return wire
}

func (service *BackupViewService) storedSlots(slots []BackupKeySlot) []backups.KeySlot {
	stored := []backups.KeySlot{}
	for _, slot := range slots {
		stored = append(stored, backups.KeySlot(slot))
	}
	return stored
}
