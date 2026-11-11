type NotificationSummaryPatch struct {
	SummaryDueAt int64  `json:"summaryDueAt"`
	SummaryState string `json:"summaryState"`
	SummarizedAt string `json:"summarizedAt"`
}
