type ThreadSendResult struct {
	Message    Message          `json:"message"`
	Deliveries []DeliveryStatus `json:"deliveries"`
}
