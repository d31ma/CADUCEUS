package login

import (
	"cmp"
	"time"

	"mesail.dev/packages/addresses"
	"mesail.dev/packages/oidc"
)

type Logins struct {
	Realm  Realm
	People People
	Signer Signer
	Now    func() time.Time
}

func (logins Logins) SignIn(login string, password string, oneTimeCode string, kept bool) (Grant, error) {
	parts, err := logins.parsed(login)
	if err != nil {
		return Grant{}, err
	}
	opened, err := logins.Realm.SignIn(parts.Username, password, oneTimeCode, kept)
	if err != nil {
		return Grant{}, err
	}
	return logins.signedIn(opened, parts.DomainId, kept)
}

func (logins Logins) PasskeySignIn(challengeKey string, credentialJson string, wanted string, kept bool) (Grant, error) {
	opened, err := logins.Realm.PasskeySignIn(challengeKey, credentialJson, kept)
	if err != nil {
		return Grant{}, err
	}
	return logins.signedIn(opened, wanted, kept)
}

func (logins Logins) Refresh(sessionKey string) (Grant, error) {
	content, err := logins.Realm.Open(sessionKey)
	if err != nil {
		return Grant{}, err
	}
	person, err := logins.People.Person(content.PersonId)
	if err != nil {
		return Grant{}, err
	}
	err = EndedSessions{}.Check(person, content.SignedInAt)
	if err != nil {
		return Grant{}, err
	}
	err = DomainRules{}.Kept(person, content.Domain)
	if err != nil {
		return Grant{}, err
	}
	return logins.refreshedInto(content, person, content.Domain)
}

func (logins Logins) Switch(sessionKey string, wanted string) (Grant, error) {
	content, err := logins.Realm.Open(sessionKey)
	if err != nil {
		return Grant{}, err
	}
	person, err := logins.People.Person(content.PersonId)
	if err != nil {
		return Grant{}, err
	}
	err = EndedSessions{}.Check(person, content.SignedInAt)
	if err != nil {
		return Grant{}, err
	}
	domainId, err := DomainRules{}.ForSwitch(person, wanted)
	if err != nil {
		return Grant{}, err
	}
	return logins.refreshedInto(content, person, domainId)
}

func (logins Logins) SignOut(sessionKey string) error {
	content, err := logins.Realm.Open(sessionKey)
	if err != nil {
		return err
	}
	return logins.Realm.SignOut(content.RefreshToken)
}

func (logins Logins) EndSessions(personId string) error {
	return logins.People.EndSessions(personId)
}

func (logins Logins) Restarted(opened oidc.KeycloakSession, domainId string, kept bool) (Grant, error) {
	err := logins.People.EndSessions(opened.PersonId)
	if err != nil {
		return Grant{}, logins.closed(opened, err)
	}
	person, err := logins.People.Person(opened.PersonId)
	if err != nil {
		return Grant{}, err
	}
	return logins.granted(opened, person, domainId, logins.started(kept))
}

func (logins Logins) signedIn(opened oidc.KeycloakSession, wanted string, kept bool) (Grant, error) {
	person, err := logins.People.Person(opened.PersonId)
	if err != nil {
		return Grant{}, logins.closed(opened, err)
	}
	domainId, err := DomainRules{}.ForSignIn(person, opened.Operator, wanted)
	if err != nil {
		return Grant{}, logins.closed(opened, err)
	}
	return logins.granted(opened, person, domainId, logins.started(kept))
}

func (logins Logins) started(kept bool) SessionStart {
	return SessionStart{Kept: kept, SignedInAt: logins.Now().UTC().Format(time.RFC3339Nano)}
}

func (logins Logins) refreshedInto(content oidc.SessionContent, person Person, domainId string) (Grant, error) {
	opened, err := logins.Realm.Refresh(content.RefreshToken)
	if err != nil {
		return Grant{}, err
	}
	err = logins.samePerson(opened, content.PersonId)
	if err != nil {
		return Grant{}, err
	}
	err = DomainRules{}.InstallWideFor(opened.Operator, domainId)
	if err != nil {
		return Grant{}, err
	}
	return logins.granted(opened, person, domainId, SessionStart{Kept: content.Kept, SignedInAt: content.SignedInAt})
}

func (logins Logins) granted(opened oidc.KeycloakSession, person Person, domainId string, start SessionStart) (Grant, error) {
	realmRoles := DomainRules{}.Roles(person, opened.Operator, domainId)
	token := oidc.MesailToken{
		Subject:         opened.PersonId,
		AuthorizedParty: oidc.AuthorizedParties{}.Web(),
		Domain:          domainId,
		RealmRoles:      realmRoles,
		Name:            cmp.Or(person.DisplayName, opened.DisplayName),
		Username:        opened.Username,
		Email:           DomainRules{}.Address(opened.Username, domainId),
		SessionId:       opened.SessionId,
	}
	issued, err := logins.Signer.Sign(token)
	if err != nil {
		return Grant{}, err
	}
	sessionKey, err := logins.Realm.Seal(oidc.SessionContent{Domain: domainId, RefreshToken: opened.RefreshToken, PersonId: opened.PersonId, Kept: start.Kept, SignedInAt: start.SignedInAt})
	return Grant{
		PersonId:           opened.PersonId,
		DomainId:           domainId,
		Username:           opened.Username,
		DisplayName:        token.Name,
		PrimaryAddress:     token.Email,
		Roles:              oidc.RoleMap{}.MesailRoles(oidc.Claims{RealmRoles: realmRoles}),
		Domains:            DomainRules{}.Choices(person),
		AccessToken:        issued.AccessToken,
		ExpiresIn:          issued.ExpiresIn,
		SessionKey:         sessionKey,
		MustChangePassword: opened.MustChangePassword,
		Kept:               start.Kept,
	}, err
}

func (Logins) parsed(login string) (addresses.LoginParts, error) {
	parts, err := addresses.Login{}.Parsed(login)
	if err != nil {
		return addresses.LoginParts{}, oidc.SignInRefused{}
	}
	return parts, nil
}

func (logins Logins) closed(opened oidc.KeycloakSession, refused error) error {
	_ = logins.Realm.SignOut(opened.RefreshToken)
	return refused
}

func (Logins) samePerson(opened oidc.KeycloakSession, personId string) error {
	if opened.PersonId != personId {
		return oidc.SignInRefused{}
	}
	return nil
}
