type PendingUndo struct {
	ID        string `json:"id"`
	UndoID    string `json:"undoId"`
	Owner     string `json:"owner"`
	Message   string `json:"message"`
	Label     string `json:"label"`
	ExpiresAt int64  `json:"expiresAt"`
}
