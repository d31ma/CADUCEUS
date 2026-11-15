package backups

type StartRequest struct {
	ExpectedVersion int64
	Size            int64
	Sha256          string
	KeySlots        []KeySlot
	DeviceId        string
}
