package recipients

import "fmt"

type RefusedDelivery struct {
	Kind   Kind
	Wanted Delivery
}

func (refused RefusedDelivery) Error() string {
	return fmt.Sprintf("delivery %q is not allowed for a %s recipient", refused.Wanted, refused.Kind)
}
