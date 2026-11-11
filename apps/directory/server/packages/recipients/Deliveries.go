package recipients

type Deliveries struct{}

func (Deliveries) Chat() Delivery {
	return "chat"
}

func (Deliveries) Email() Delivery {
	return "email"
}

func (Deliveries) Both() Delivery {
	return "both"
}
