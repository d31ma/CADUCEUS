package adapters

import "fmt"

type UnexpectedStatus struct {
	Service string
	Status  int
}

func (failure UnexpectedStatus) Error() string {
	return fmt.Sprintf("%s answered HTTP %d", failure.Service, failure.Status)
}
