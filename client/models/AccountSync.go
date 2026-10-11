type AccountSync struct {
	CursorVersion     int    `json:"cursorVersion"`
	Attempts          int    `json:"attempts"`
	NextRunAt         string `json:"nextRunAt"`
	LastCode          string `json:"lastCode"`
	ScheduleUpdatedAt string `json:"scheduleUpdatedAt"`
}
