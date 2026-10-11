type AccountDiagnostics struct {
	CursorVersion int    `json:"cursorVersion"`
	Attempts      int    `json:"attempts"`
	NextRunAt     string `json:"nextRunAt"`
	LastCode      string `json:"lastCode"`
	UpdatedAt     string `json:"updatedAt"`
}
