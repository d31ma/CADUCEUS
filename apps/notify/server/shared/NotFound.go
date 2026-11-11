type NotFound struct {
	What string
}

func (failure *NotFound) Error() string {
	return "the " + failure.What + " was not found"
}
