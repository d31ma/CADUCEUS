import "errors"

type BatchUtils struct{}

func (BatchUtils) Each(count int, step func(index int) error) error {
	return BatchUtils{}.each(count, step)
}

func (BatchUtils) each(count int, step func(index int) error) error {
	failures := []error{}
	for index := range count {
		failures = append(failures, step(index))
	}
	return errors.Join(failures...)
}
