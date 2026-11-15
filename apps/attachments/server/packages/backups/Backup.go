package backups

type Backup struct {
	Version           int64
	Size              int64
	Sha256            string
	KeySlots          []KeySlot
	UpdatedAt         string
	UpdatedByDeviceId string
}
