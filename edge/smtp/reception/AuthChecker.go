package reception

import "context"

type AuthChecker interface {
	Check(ctx context.Context, input AuthInput) AuthVerdict
}
