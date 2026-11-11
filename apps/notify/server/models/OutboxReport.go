type OutboxReport struct {
	Documents    int `json:"documents"`
	Delivered    int `json:"delivered"`
	Retried      int `json:"retried"`
	DeadLettered int `json:"deadLettered"`
}
