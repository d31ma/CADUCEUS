package backend

import "fmt"

type UnexpectedStatus struct {
	Path   string
	Status int
}

func (problem UnexpectedStatus) Error() string {
	return fmt.Sprintf("backend %s answered %d", problem.Path, problem.Status)
}
