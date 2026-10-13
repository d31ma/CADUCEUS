type MailActionRequest struct {
	Ids             []string       `json:"ids"`
	Patch           map[string]any `json:"patch"`
	ClientRequestID string         `json:"clientRequestId"`
}
