package events

import (
	"slices"
	"strings"
)

type DomainName struct{}

func (name DomainName) Check(domainId string) error {
	err := name.checkLength(domainId)
	if err != nil {
		return err
	}
	err = name.checkCharacters(domainId)
	if err != nil {
		return err
	}
	err = name.checkDot(domainId)
	if err != nil {
		return err
	}
	return name.checkLabels(domainId)
}

func (DomainName) maximumLength() int {
	return 253
}

func (DomainName) allowedCharacters() string {
	return "abcdefghijklmnopqrstuvwxyz0123456789-."
}

func (DomainName) dot() string {
	return "."
}

func (name DomainName) checkLength(domainId string) error {
	if len(domainId) > name.maximumLength() {
		return InvalidEnvelope{Reason: "domainId is longer than 253 characters"}
	}
	return nil
}

func (name DomainName) checkCharacters(domainId string) error {
	if strings.ContainsFunc(domainId, name.notAllowed) {
		return InvalidEnvelope{Reason: "domainId may only hold the characters a-z, 0-9, - and ."}
	}
	return nil
}

func (name DomainName) notAllowed(character rune) bool {
	return !strings.ContainsRune(name.allowedCharacters(), character)
}

func (name DomainName) checkDot(domainId string) error {
	if !strings.Contains(domainId, name.dot()) {
		return InvalidEnvelope{Reason: "domainId must hold at least one dot"}
	}
	return nil
}

func (name DomainName) checkLabels(domainId string) error {
	if slices.Contains(strings.Split(domainId, name.dot()), "") {
		return InvalidEnvelope{Reason: "domainId must not hold an empty label"}
	}
	return nil
}
