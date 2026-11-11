type SummaryBatch struct {
	MemberId string               `json:"memberId"`
	Records  []NotificationRecord `json:"records"`
}
