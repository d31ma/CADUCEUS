package channels

import "mesail.dev/packages/outbox"

type SweepReport struct {
	Documents    int  `json:"documents"`
	Delivered    int  `json:"delivered"`
	Retried      int  `json:"retried"`
	DeadLettered int  `json:"deadLettered"`
	More         bool `json:"more"`
}

func (report SweepReport) Plus(part outbox.Report) SweepReport {
	report.Documents += part.Documents
	report.Delivered += part.Delivered
	report.Retried += part.Retried
	report.DeadLettered += part.DeadLettered
	return report
}
