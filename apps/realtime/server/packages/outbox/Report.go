package outbox

type Report struct {
	Documents    int `json:"documents"`
	Delivered    int `json:"delivered"`
	Retried      int `json:"retried"`
	DeadLettered int `json:"deadLettered"`
}

func (report Report) plus(other Report) Report {
	return Report{
		Documents:    report.Documents + other.Documents,
		Delivered:    report.Delivered + other.Delivered,
		Retried:      report.Retried + other.Retried,
		DeadLettered: report.DeadLettered + other.DeadLettered,
	}
}
