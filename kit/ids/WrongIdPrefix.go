package ids

import "fmt"

type WrongIdPrefix struct {
	Expected string
}

func (wrong WrongIdPrefix) Error() string {
	return fmt.Sprintf("the id does not start with %q", wrong.Expected)
}
