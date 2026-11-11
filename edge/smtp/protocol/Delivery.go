package protocol

import (
	"context"

	"mesail.dev/edge/smtp/reception"
)

type Delivery interface {
	Deliver(ctx context.Context, transaction reception.Transaction) (reception.Receipt, error)
}
