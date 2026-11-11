type SendResult struct {
	Message    Message          `json:"message"`
	Deliveries []DeliveryStatus `json:"deliveries"`
}
