package recipients

import "fmt"

type InvalidDomainName struct {
	Name string
}

func (invalid InvalidDomainName) Error() string {
	return fmt.Sprintf("%q is not a valid domain name", invalid.Name)
}
