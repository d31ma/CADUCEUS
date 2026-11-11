type RealmNotManaged struct {
	Message string
}

func (failure *RealmNotManaged) Error() string {
	return failure.Message
}
