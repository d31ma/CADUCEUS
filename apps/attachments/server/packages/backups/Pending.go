package backups

type Pending struct {
	Version    int64    `json:"version"`
	Blob       string   `json:"blob"`
	Size       int64    `json:"size"`
	Sha256     string   `json:"sha256"`
	KeySlots   []string `json:"keySlots"`
	DeviceId   string   `json:"deviceId"`
	CompleteBy string   `json:"completeBy"`
}
