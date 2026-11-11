type StorageUnavailable struct {
	Operation string
}

func (failure *StorageUnavailable) Error() string {
	return "notify storage failed: " + failure.Operation
}
