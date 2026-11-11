package mailchecks

import (
	"context"

	"mesail.dev/packages/mailauth"
)

type Checker interface {
	Check(ctx context.Context, input mailauth.Input) mailauth.Results
}
