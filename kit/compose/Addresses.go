package compose

import (
	"net/mail"
	"strings"
)

type Addresses struct{}

func (addresses Addresses) Parse(text string) (Address, error) {
	parsed, err := mail.ParseAddress(text)
	if err != nil {
		return Address{}, err
	}
	return Address{Name: parsed.Name, Email: addresses.lowerDomain(parsed.Address)}, nil
}

func (Addresses) Domain(email string) string {
	return strings.ToLower(email[strings.LastIndex(email, "@")+1:])
}

func (Addresses) lowerDomain(email string) string {
	at := strings.LastIndex(email, "@")
	return email[:at] + strings.ToLower(email[at:])
}
