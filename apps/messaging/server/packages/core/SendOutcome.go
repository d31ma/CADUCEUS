package core

type SendOutcome struct {
	Message    MessageView
	Deliveries []DeliveryRecord
}
