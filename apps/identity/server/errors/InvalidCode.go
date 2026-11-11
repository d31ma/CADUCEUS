type InvalidCode struct{}

func (failure *InvalidCode) Error() string {
	return "the 2-step code is not right"
}
