package oidc

import (
	"cmp"
	"encoding/base64"
	"encoding/json"
)

type PasskeyOptions struct {
	RelyingPartyId string
}

func (options PasskeyOptions) Request(challenge string) (string, error) {
	request := PasskeyRequestOptions{
		Challenge:        challenge,
		RelyingPartyId:   options.RelyingPartyId,
		Timeout:          options.timeoutMilliseconds(),
		UserVerification: options.required(),
		AllowCredentials: []PasskeyDescriptor{},
	}
	return options.text(request)
}

func (options PasskeyOptions) Creation(challenge string, caller Caller, existing []PasskeyRecord) (string, error) {
	creation := PasskeyCreationOptions{
		Challenge:              challenge,
		RelyingParty:           PasskeyRelyingParty{Id: options.RelyingPartyId, Name: options.relyingPartyName()},
		User:                   options.user(caller),
		Parameters:             options.parameters(),
		Timeout:                options.timeoutMilliseconds(),
		ExcludeCredentials:     options.excluded(existing),
		AuthenticatorSelection: PasskeySelection{ResidentKey: options.required(), RequireResidentKey: true, UserVerification: options.required()},
		Attestation:            "none",
	}
	return options.text(creation)
}

func (options PasskeyOptions) user(caller Caller) PasskeyUser {
	userHandle := base64.RawURLEncoding.EncodeToString([]byte(caller.MemberId))
	return PasskeyUser{Id: userHandle, Name: caller.Username, DisplayName: cmp.Or(caller.DisplayName, caller.Username)}
}

func (options PasskeyOptions) parameters() []PasskeyParameter {
	return []PasskeyParameter{{Type: options.credentialType(), Algorithm: -7}, {Type: options.credentialType(), Algorithm: -257}}
}

func (options PasskeyOptions) excluded(existing []PasskeyRecord) []PasskeyDescriptor {
	descriptors := []PasskeyDescriptor{}
	for _, record := range existing {
		descriptors = append(descriptors, PasskeyDescriptor{Type: options.credentialType(), Id: record.CredentialId})
	}
	return descriptors
}

func (PasskeyOptions) text(value any) (string, error) {
	raw, err := json.Marshal(value)
	return string(raw), err
}

func (PasskeyOptions) credentialType() string {
	return "public-key"
}

func (PasskeyOptions) required() string {
	return "required"
}

func (PasskeyOptions) relyingPartyName() string {
	return "Mesail"
}

func (PasskeyOptions) timeoutMilliseconds() int64 {
	return 120000
}
