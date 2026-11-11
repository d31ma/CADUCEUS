package realtime

type Missing struct {
	What string
}

func (failure Missing) Error() string {
	return "not found: " + failure.What
}
