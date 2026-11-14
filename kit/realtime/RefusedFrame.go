package realtime

type RefusedFrame struct {
	Type        string `json:"type"`
	RequestType string `json:"requestType"`
	RequestId   string `json:"requestId,omitempty"`
	State       string `json:"state"`
	Field       string `json:"field,omitempty"`
	Message     string `json:"message"`
}
