type NotFound struct {
	What string
}

func (failure *NotFound) Error() string {
	return "not found: " + failure.What
}
