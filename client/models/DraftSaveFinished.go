type DraftSaveFinished struct {
	Status        string `json:"status"`
	DraftID       string `json:"draftId"`
	DraftClientID string `json:"draftClientId"`
	DraftVersion  int    `json:"draftVersion"`
	Message       string `json:"message"`
}
