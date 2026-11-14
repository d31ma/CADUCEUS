type StorageUnavailable struct {
	ErrorCase
	Operation string
}

func (failure *StorageUnavailable) Error() string {
	return "notify storage failed: " + failure.Operation
}

func (StorageUnavailable) Store(operation string) *StorageUnavailable {
	return &StorageUnavailable{ErrorCase{4517, "unavailable", "", "Notification storage is not available. Try again later."}, operation}
}

func (StorageUnavailable) OutboxSettings() *StorageUnavailable {
	return &StorageUnavailable{ErrorCase{4518, "unavailable", "", "Notification storage is not available. Try again later."}, "read the outbox settings"}
}

func (StorageUnavailable) OutboxRelay() *StorageUnavailable {
	return &StorageUnavailable{ErrorCase{4519, "unavailable", "", "Notification storage is not available. Try again later."}, "relay the outbox"}
}
