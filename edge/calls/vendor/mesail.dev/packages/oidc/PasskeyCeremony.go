package oidc

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"time"
)

type PasskeyCeremony struct {
	Sealer  SealedCookie
	Origins PasskeyOrigins
	Now     func() time.Time
}

func (ceremony PasskeyCeremony) Challenge() (PasskeyChallenge, error) {
	challenge, err := ceremony.newChallenge()
	if err != nil {
		return PasskeyChallenge{}, err
	}
	optionsJson, err := ceremony.options().Request(challenge)
	if err != nil {
		return PasskeyChallenge{}, err
	}
	challengeKey, err := ceremony.seal(PasskeyChallengeContent{Challenge: challenge, ExpiresAt: ceremony.expiry(ceremony.signInSeconds())}, ceremony.signInLabel())
	return PasskeyChallenge{OptionsJson: optionsJson, ChallengeKey: challengeKey}, err
}

func (ceremony PasskeyCeremony) SignInChallenge(challengeKey string) (string, error) {
	content, err := ceremony.opened(challengeKey, ceremony.signInLabel(), SignInRefused{})
	if err != nil {
		return "", err
	}
	return content.Challenge, ceremony.unexpired(content, SignInRefused{})
}

func (ceremony PasskeyCeremony) Setup(caller Caller, existing []PasskeyRecord) (PasskeySetup, error) {
	challenge, err := ceremony.newChallenge()
	if err != nil {
		return PasskeySetup{}, err
	}
	optionsJson, err := ceremony.options().Creation(challenge, caller, existing)
	if err != nil {
		return PasskeySetup{}, err
	}
	setupKey, err := ceremony.seal(PasskeyChallengeContent{Challenge: challenge, Subject: caller.MemberId, ExpiresAt: ceremony.expiry(ceremony.setupSeconds())}, ceremony.setupLabel())
	return PasskeySetup{OptionsJson: optionsJson, SetupKey: setupKey}, err
}

func (ceremony PasskeyCeremony) SetupChallenge(caller Caller, setupKey string) (string, error) {
	content, err := ceremony.opened(setupKey, ceremony.setupLabel(), InvalidPasskey{})
	if err != nil {
		return "", err
	}
	err = ceremony.owned(content, caller)
	if err != nil {
		return "", err
	}
	return content.Challenge, ceremony.unexpired(content, SetupExpired{})
}

func (ceremony PasskeyCeremony) CheckedCredential(credentialJson string) error {
	return ceremony.sized(len(credentialJson))
}

func (ceremony PasskeyCeremony) options() PasskeyOptions {
	return PasskeyOptions{RelyingPartyId: ceremony.Origins.RelyingPartyId()}
}

func (ceremony PasskeyCeremony) newChallenge() (string, error) {
	raw := make([]byte, ceremony.challengeBytes())
	_, err := rand.Read(raw)
	return base64.RawURLEncoding.EncodeToString(raw), err
}

func (ceremony PasskeyCeremony) seal(content PasskeyChallengeContent, label string) (string, error) {
	plain, err := json.Marshal(content)
	if err != nil {
		return "", err
	}
	return ceremony.Sealer.Seal(string(plain), label)
}

func (ceremony PasskeyCeremony) opened(sealed string, label string, refused error) (PasskeyChallengeContent, error) {
	plain, err := ceremony.Sealer.Open(sealed, label)
	if err != nil {
		return PasskeyChallengeContent{}, refused
	}
	return ceremony.decoded(plain, refused)
}

func (PasskeyCeremony) decoded(plain string, refused error) (PasskeyChallengeContent, error) {
	content := PasskeyChallengeContent{}
	if json.Unmarshal([]byte(plain), &content) != nil {
		return PasskeyChallengeContent{}, refused
	}
	return content, nil
}

func (PasskeyCeremony) owned(content PasskeyChallengeContent, caller Caller) error {
	if content.Subject != caller.MemberId {
		return InvalidPasskey{}
	}
	return nil
}

func (ceremony PasskeyCeremony) unexpired(content PasskeyChallengeContent, expired error) error {
	if ceremony.Now().Unix() >= content.ExpiresAt {
		return expired
	}
	return nil
}

func (ceremony PasskeyCeremony) sized(length int) error {
	if length > ceremony.largestCredential() {
		return SignInRefused{}
	}
	return nil
}

func (ceremony PasskeyCeremony) expiry(seconds int64) int64 {
	return ceremony.Now().Unix() + seconds
}

func (PasskeyCeremony) challengeBytes() int {
	return 32
}

func (PasskeyCeremony) largestCredential() int {
	return 64 * 1024
}

func (PasskeyCeremony) signInSeconds() int64 {
	return 120
}

func (PasskeyCeremony) setupSeconds() int64 {
	return 600
}

func (PasskeyCeremony) signInLabel() string {
	return "mesail-passkey-challenge"
}

func (PasskeyCeremony) setupLabel() string {
	return "mesail-passkey-setup"
}
