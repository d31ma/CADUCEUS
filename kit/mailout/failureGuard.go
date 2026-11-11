package mailout

type failureGuard[T any] struct {
	failure error
}

func (guard failureGuard[T]) run(work func() (T, error)) (T, error) {
	if guard.failure != nil {
		var empty T
		return empty, guard.failure
	}
	return work()
}
