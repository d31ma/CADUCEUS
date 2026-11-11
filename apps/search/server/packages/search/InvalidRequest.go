package search

type InvalidRequest struct {
	Message string
	Field   string
}

func (failure InvalidRequest) Error() string {
	return failure.Message
}
