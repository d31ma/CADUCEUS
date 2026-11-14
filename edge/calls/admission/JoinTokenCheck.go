package admission

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	"mesail.dev/edge/calls/failures"
	"mesail.dev/packages/oidc"
)

type JoinTokenCheck struct {
	keys  KeyLookup
	clock Clock
}

func NewJoinTokenCheck(keys KeyLookup, clock Clock) JoinTokenCheck {
	return JoinTokenCheck{keys: keys, clock: clock}
}

func (check JoinTokenCheck) Admit(rawToken string, callId string) (JoinClaims, error) {
	claims, err := check.Identify(rawToken, callId)
	if err != nil {
		return JoinClaims{}, err
	}
	return claims, check.unexpired(claims)
}

func (check JoinTokenCheck) Identify(rawToken string, callId string) (JoinClaims, error) {
	token, err := check.parsed(rawToken)
	if err != nil {
		return JoinClaims{}, err
	}
	key, err := check.keys.Key(token.Header.KeyId)
	if err != nil {
		return JoinClaims{}, err
	}
	err = check.verified(token, key)
	if err != nil {
		return JoinClaims{}, err
	}
	claims, err := check.claims(token)
	if err != nil {
		return JoinClaims{}, err
	}
	err = check.complete(claims)
	if err != nil {
		return JoinClaims{}, err
	}
	return claims, check.forCall(claims, callId)
}

func (check JoinTokenCheck) parsed(rawToken string) (oidc.SignedToken, error) {
	if rawToken == "" {
		return oidc.SignedToken{}, failures.Problems{}.MissingToken()
	}
	return check.parsedText(rawToken)
}

func (JoinTokenCheck) parsedText(rawToken string) (oidc.SignedToken, error) {
	token, err := oidc.TokenParser{}.Parse(rawToken)
	if err != nil {
		return oidc.SignedToken{}, failures.Problems{}.InvalidToken()
	}
	return token, nil
}

func (JoinTokenCheck) verified(token oidc.SignedToken, key oidc.JsonWebKey) error {
	if (oidc.SignatureCheck{}).Verify(token, key) != nil {
		return failures.Problems{}.InvalidToken()
	}
	return nil
}

func (check JoinTokenCheck) claims(token oidc.SignedToken) (JoinClaims, error) {
	payload := token.SigningInput[strings.IndexByte(token.SigningInput, '.')+1:]
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return JoinClaims{}, failures.Problems{}.InvalidToken()
	}
	return check.decoded(raw)
}

func (JoinTokenCheck) decoded(raw []byte) (JoinClaims, error) {
	claims := JoinClaims{}
	if json.Unmarshal(raw, &claims) != nil {
		return JoinClaims{}, failures.Problems{}.InvalidToken()
	}
	return claims, nil
}

func (check JoinTokenCheck) complete(claims JoinClaims) error {
	if claims.Audience != check.audience() || claims.DeviceId == "" || claims.DomainId == "" || claims.ExpiresAt == 0 {
		return failures.Problems{}.InvalidToken()
	}
	return nil
}

func (JoinTokenCheck) forCall(claims JoinClaims, callId string) error {
	if claims.CallId != callId {
		return failures.Problems{}.TokenForAnotherCall()
	}
	return nil
}

func (check JoinTokenCheck) unexpired(claims JoinClaims) error {
	if check.clock.Now().Unix() >= claims.ExpiresAt {
		return failures.Problems{}.ExpiredToken()
	}
	return nil
}

func (JoinTokenCheck) audience() string {
	return "calls"
}
