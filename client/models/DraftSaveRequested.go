type DraftSaveRequested struct {
	Envelope      MessageEnvelope `json:"envelope"`
	DraftID       string          `json:"draftId"`
	DraftClientID string          `json:"draftClientId"`
	DraftVersion  int             `json:"draftVersion"`
}
