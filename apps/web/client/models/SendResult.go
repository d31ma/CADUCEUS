type SendResult struct {
	Message    SendResultMessage `json:"message"`
	Deliveries []SendDelivery    `json:"deliveries"`
}
