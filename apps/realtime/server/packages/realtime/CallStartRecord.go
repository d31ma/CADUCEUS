package realtime

type CallStartRecord struct {
	ID        string   `json:"id,omitempty"`
	DeviceId  string   `json:"deviceId"`
	StartedAt []string `json:"startedAt"`
}
