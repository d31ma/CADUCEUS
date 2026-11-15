type Backup struct {
	Version           int64           `json:"version"`
	Size              int64           `json:"size"`
	Sha256            string          `json:"sha256"`
	KeySlots          []BackupKeySlot `json:"keySlots"`
	UpdatedAt         string          `json:"updatedAt"`
	UpdatedByDeviceId string          `json:"updatedByDeviceId"`
}
