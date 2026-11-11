package oidc

import "time"

type Gate struct {
	verifier TokenVerifier
	now      func() time.Time
}

func NewGate(verifier TokenVerifier, now func() time.Time) Gate {
	return Gate{verifier: verifier, now: now}
}

func Configured() Gate {
	return NewGate(VerifierFactory{}.FromEnvironment(), time.Now)
}

func (gate Gate) Identify(authorization string) (Caller, error) {
	token, err := BearerToken{}.FromHeader(authorization)
	if err != nil {
		return Caller{}, err
	}
	claims, err := gate.verifier.Verify(token, gate.now())
	if err != nil {
		return Caller{}, err
	}
	return gate.caller(claims)
}

func (gate Gate) caller(claims Claims) (Caller, error) {
	domainId, err := Realm{}.Name(claims.Issuer)
	caller := Caller{DomainId: domainId, MemberId: claims.Subject, DisplayName: claims.Name, Email: claims.Email, Roles: RoleMap{}.MesailRoles(claims)}
	return caller, err
}
