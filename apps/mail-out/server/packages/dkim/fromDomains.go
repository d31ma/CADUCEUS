package dkim

import (
	"net/mail"
	"strings"
)

type fromDomains struct{}

func (domains fromDomains) check(parsed message, key Key) error {
	text := parsed.fromText()
	addresses, err := mail.ParseAddressList(text)
	if err != nil {
		return DomainMismatch{From: strings.TrimSpace(text), Key: key.Domain}
	}
	return domains.mismatch(addresses, key.Domain)
}

func (domains fromDomains) mismatch(addresses []*mail.Address, keyDomain string) error {
	for _, address := range addresses {
		if !domains.within(domains.domain(address.Address), keyDomain) {
			return DomainMismatch{From: address.Address, Key: keyDomain}
		}
	}
	return nil
}

func (fromDomains) domain(email string) string {
	return strings.ToLower(email[strings.LastIndex(email, "@")+1:])
}

func (fromDomains) within(domain string, keyDomain string) bool {
	return strings.HasSuffix("."+domain, "."+strings.ToLower(keyDomain))
}
