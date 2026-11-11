package attachments

import "mesail.dev/packages/oidc"

type CallerGate interface {
	Identify(authorization string) (oidc.Caller, error)
}
