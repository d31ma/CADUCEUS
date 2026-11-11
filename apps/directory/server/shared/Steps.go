type Steps struct{}

func (Steps) Unless(failure error, step func() error) error {
	if failure != nil {
		return failure
	}
	return step()
}
