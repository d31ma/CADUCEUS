type SendFinished struct {
	Mode     string `json:"mode"`
	Status   string `json:"status"`
	UndoID   string `json:"undoId"`
	Archived bool   `json:"archived"`
	Message  string `json:"message"`
	SendAt   string `json:"sendAt"`
}
