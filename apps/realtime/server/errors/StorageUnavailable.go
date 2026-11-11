type StorageUnavailable struct {
	Operation string
}

func (failure *StorageUnavailable) Error() string {
	return "realtime storage failed: " + failure.Operation
}
