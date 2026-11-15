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
		return InvalidEnvelope{}.DomainTooLong()
	}
	return nil
}

func (name DomainName) checkCharacters(domainId string) error {
	if strings.ContainsFunc(domainId, name.notAllowed) {
		return InvalidEnvelope{}.DomainCharacters()
	}
	return nil
}

func (name DomainName) notAllowed(character rune) bool {
	return !strings.ContainsRune(name.allowedCharacters(), character)
}

func (name DomainName) checkDot(domainId string) error {
	if !strings.Contains(domainId, name.dot()) {
		return InvalidEnvelope{}.DomainWithoutDot()
	}
	return nil
}

func (name DomainName) checkLabels(domainId string) error {
	if slices.Contains(strings.Split(domainId, name.dot()), "") {
		return InvalidEnvelope{}.DomainEmptyLabel()
	}
	return nil
}
