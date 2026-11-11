type StorageUnavailable struct {
	Operation string
}

func (failure *StorageUnavailable) Error() string {
	return "inbound storage failed: " + failure.Operation
}
