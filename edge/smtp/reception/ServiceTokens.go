package reception

import "context"

type ServiceTokens interface {
	Token(ctx context.Context) (string, error)
}
