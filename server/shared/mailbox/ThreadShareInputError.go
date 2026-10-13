// ThreadShareInputError refuses a share request the caller can correct.
type ThreadShareInputError struct {
	Message string
}

func (failure *ThreadShareInputError) Error() string { return failure.Message }
