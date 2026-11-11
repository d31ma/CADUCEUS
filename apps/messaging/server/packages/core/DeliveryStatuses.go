package core

type DeliveryStatuses struct{}

func (DeliveryStatuses) Delivered() string {
	return "delivered"
}

func (DeliveryStatuses) Read() string {
	return "read"
}

func (DeliveryStatuses) Queued() string {
	return "queued"
}

func (DeliveryStatuses) Sent() string {
	return "sent"
}

func (DeliveryStatuses) Deferred() string {
	return "deferred"
}

func (DeliveryStatuses) Bounced() string {
	return "bounced"
}

func (DeliveryStatuses) Failed() string {
	return "failed"
}
