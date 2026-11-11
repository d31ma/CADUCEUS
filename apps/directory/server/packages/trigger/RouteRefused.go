package trigger

import "fmt"

type RouteRefused struct {
	Status int
}

func (refused RouteRefused) Error() string {
	return fmt.Sprintf("the scheduled route answered %d", refused.Status)
}
