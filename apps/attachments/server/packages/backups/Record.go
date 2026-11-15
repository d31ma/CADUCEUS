package backups

type Record struct {
	ID                string   `json:"id,omitempty"`
	DomainId          string   `json:"domainId"`
	PersonId          string   `json:"personId"`
	Version           int64    `json:"version"`
	Blob              string   `json:"blob"`
	Size              int64    `json:"size"`
	Sha256            string   `json:"sha256"`
	KeySlots          []string `json:"keySlots"`
	UpdatedAt         string   `json:"updatedAt"`
	UpdatedByDeviceId string   `json:"updatedByDeviceId"`
	Pending           Pending  `json:"pending"`
}
