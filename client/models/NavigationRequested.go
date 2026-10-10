type NavigationRequested struct {
	Route         string `json:"route"`
	ClearRecovery bool   `json:"clearRecovery"`
	DiscardDraft  string `json:"discardDraft"`
	UndoID        string `json:"undoId"`
	UndoLabel     string `json:"undoLabel"`
	UndoDelay     int    `json:"undoDelay"`
	Toast         string `json:"toast"`
	ToastDate     string `json:"toastDate"`
}
