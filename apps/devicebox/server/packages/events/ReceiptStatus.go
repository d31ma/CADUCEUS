package events

type ReceiptStatus struct{}

func (ReceiptStatus) Applied() string   { return "applied" }
func (ReceiptStatus) Duplicate() string { return "duplicate" }
