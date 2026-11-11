package mime

import (
	"net/mail"
	"strings"
)

type addressList struct{}

func (list addressList) All(value string) []Mailbox {
	parser := mail.AddressParser{WordDecoder: encodedWords{}.decoder()}
	parsed, err := parser.ParseList(value)
	return list.mailboxes(parsed, err)
}

func (list addressList) First(value string) Mailbox {
	return list.first(list.All(value))
}

func (list addressList) mailboxes(parsed []*mail.Address, err error) []Mailbox {
	if err != nil {
		return nil
	}
	return list.converted(parsed)
}

func (list addressList) converted(parsed []*mail.Address) []Mailbox {
	mailboxes := make([]Mailbox, 0, len(parsed))
	for _, address := range parsed {
		mailboxes = append(mailboxes, list.mailbox(address))
	}
	return mailboxes
}

func (list addressList) mailbox(address *mail.Address) Mailbox {
	text := charsets{}
	return Mailbox{Name: text.Valid(address.Name), Address: text.Valid(list.lowerDomain(address.Address))}
}

func (addressList) lowerDomain(address string) string {
	at := strings.LastIndex(address, "@") + 1
	return address[:at] + strings.ToLower(address[at:])
}

func (addressList) first(mailboxes []Mailbox) Mailbox {
	if len(mailboxes) == 0 {
		return Mailbox{}
	}
	return mailboxes[0]
}
