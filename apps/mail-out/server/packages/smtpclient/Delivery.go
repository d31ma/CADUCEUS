package smtpclient

import "context"

type Delivery interface {
	Deliver(ctx context.Context, envelope Envelope, message []byte) (Reply, error)
}
