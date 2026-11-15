package recipients

import "fmt"

type InvalidAddress struct {
	Address string
}

func (invalid InvalidAddress) Error() string {
	return fmt.Sprintf("%q is not a valid email address", invalid.Address)
}

func (InvalidAddress) Number() int {
	return 31
}
