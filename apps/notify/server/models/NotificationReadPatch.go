type NotificationReadPatch struct {
	ReadAt       string `json:"readAt"`
	SummaryDueAt int64  `json:"summaryDueAt"`
	SummaryState string `json:"summaryState"`
	PushState    string `json:"pushState"`
}
