package webpush

type Outcomes struct{}

func (Outcomes) Delivered() string {
	return "delivered"
}

func (Outcomes) Gone() string {
	return "gone"
}

func (Outcomes) Retry() string {
	return "retry"
}

func (Outcomes) Refused() string {
	return "refused"
}
