package realtime

type SignalResult struct {
	Type      string   `json:"type"`
	RequestId string   `json:"requestId,omitempty"`
	Delivered []string `json:"delivered"`
	Offline   []string `json:"offline"`
}
