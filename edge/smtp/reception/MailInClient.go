package reception

import "context"

type MailInClient interface {
	Accept(ctx context.Context, envelope InboundEnvelope) (string, error)
}
