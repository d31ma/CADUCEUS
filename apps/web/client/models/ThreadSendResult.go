type ThreadSendResult struct {
	Message    ThreadMessage    `json:"message"`
	Deliveries []ThreadDelivery `json:"deliveries"`
}
