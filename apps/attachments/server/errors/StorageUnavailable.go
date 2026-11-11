type StorageUnavailable struct {
	Operation string
}

func (failure *StorageUnavailable) Error() string {
	return "attachment storage failed during " + failure.Operation
}
