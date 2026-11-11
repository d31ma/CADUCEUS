package attachments

type States struct{}

func (States) Pending() string {
	return "pending"
}

func (States) Ready() string {
	return "ready"
}

func (States) Blocked() string {
	return "blocked"
}
