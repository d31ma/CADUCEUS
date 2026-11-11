type NotFound struct{}

func (failure *NotFound) Error() string {
	return "not found"
}
