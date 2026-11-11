package reception

import "context"

type AddressDirectory interface {
	Lookup(ctx context.Context, address string) (Address, error)
}
