package startup

import "fmt"

type ShortSecret struct {
	Name    string
	Minimum int
}

func (problem ShortSecret) Error() string {
	return fmt.Sprintf("%s must be set and at least %d characters long", problem.Name, problem.Minimum)
}
