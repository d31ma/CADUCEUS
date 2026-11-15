type BackupStart struct {
	ExpectedVersion int64           `json:"expectedVersion"`
	Size            int64           `json:"size"`
	Sha256          string          `json:"sha256"`
	KeySlots        []BackupKeySlot `json:"keySlots"`
	DeviceId        string          `json:"deviceId"`
}
