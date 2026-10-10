type AccountDiagnostics struct {
	CursorVersion float64 `json:"cursorVersion"`
	Attempts      float64 `json:"attempts"`
	NextRunAt     string  `json:"nextRunAt"`
	LastCode      string  `json:"lastCode"`
	UpdatedAt     string  `json:"updatedAt"`
}
