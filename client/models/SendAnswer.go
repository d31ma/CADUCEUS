// What POST /send and PUT /send/scheduled/{id} answer.
type SendAnswer struct {
	UndoID    string `json:"undoId"`
	Archived  bool   `json:"archived"`
	Queued    bool   `json:"queued"`
	MessageID string `json:"messageId"`
}
