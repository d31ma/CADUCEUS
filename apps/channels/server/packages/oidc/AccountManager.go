package oidc

import (
	"encoding/json"

	"slices"
	"time"
)

type AccountManager struct {
	Sessions SessionIssuer
	Now      func() time.Time
}

func (manager AccountManager) setup(caller Caller, secret string) (TotpSetup, error) {
	uri := Totp{}.Uri(caller.Email, secret)
	image, _ := QrCode{}.DataUri(uri)
	content, err := json.Marshal(TotpSetupContent{Secret: secret, Subject: caller.MemberId, Domain: caller.DomainId, ExpiresAt: manager.Now().Unix() + manager.setupSeconds()})
	if err != nil {
		return TotpSetup{}, err
	}
	setupKey, err := manager.Sessions.Sealer.Seal(string(content), manager.setupLabel())
	return TotpSetup{Secret: secret, Uri: uri, QrImage: image, SetupKey: setupKey}, err
}

func (manager AccountManager) openedSetup(setupKey string) (TotpSetupContent, error) {
	plain, err := manager.Sessions.Sealer.Open(setupKey, manager.setupLabel())
	if err != nil {
		return TotpSetupContent{}, InvalidCode{}
	}
	content := TotpSetupContent{}
	return content, json.Unmarshal([]byte(plain), &content)
}

func (manager AccountManager) validSetup(caller Caller, content TotpSetupContent, code string) error {
	if content.Subject != caller.MemberId || content.Domain != caller.DomainId || manager.Now().Unix() >= content.ExpiresAt || !(Totp{}).Matches(content.Secret, code, manager.Now()) {
		return InvalidCode{}
	}
	return nil
}

func (manager AccountManager) endOtherSessions(admin KeycloakAdmin, userId string, keep string) error {
	sessions, err := admin.Sessions(userId)
	if err != nil {
		return err
	}
	others := slices.DeleteFunc(sessions, func(session UserSession) bool { return session.Id == keep })
	return manager.endSessions(admin, others)
}

func (AccountManager) endSessions(admin KeycloakAdmin, sessions []UserSession) error {
	for _, session := range sessions {
		err := admin.EndSession(session.Id)
		if err != nil {
			return err
		}
	}
	return nil
}

func (AccountManager) deleteOtp(admin KeycloakAdmin, userId string, credentials []UserCredential) error {
	for _, credential := range credentials {
		err := admin.DeleteCredential(userId, credential.Id)
		if err != nil {
			return err
		}
	}
	return nil
}

func (manager AccountManager) isOtp(credential UserCredential) bool {
	return credential.Type == manager.otpType()
}

func (manager AccountManager) isNotOtp(credential UserCredential) bool {
	return credential.Type != manager.otpType()
}

func (AccountManager) otpType() string {
	return "otp"
}

func (AccountManager) setupLabel() string {
	return "mesail-totp-setup"
}

func (AccountManager) setupSeconds() int64 {
	return 600
}
