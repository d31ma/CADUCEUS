package realtime

import "mesail.dev/packages/oidc"

type CallerGate struct {
	Gate oidc.Gate
}

func (gate CallerGate) Caller(authorization string) (oidc.Caller, error) {
	caller, err := gate.Gate.Identify(authorization)
	return caller, gate.refusal(err)
}

func (CallerGate) refusal(err error) error {
	if err != nil {
		return Unauthenticated{}
	}
	return nil
}
