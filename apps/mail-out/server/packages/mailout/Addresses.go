package mailout

import (
	"net/mail"
	"strings"
)

type Addresses struct{}

func (Addresses) Domain(email string) string {
	return strings.ToLower(email[strings.LastIndex(email, "@")+1:])
}

func (Addresses) Mailbox(text string) MailboxAddress {
	address, err := mail.ParseAddress(text)
	if err != nil {
		return MailboxAddress{}
	}
	return MailboxAddress{Name: address.Name, Address: address.Address}
}

func (addresses Addresses) Mailboxes(texts []string) []MailboxAddress {
	mailboxes := []MailboxAddress{}
	for _, text := range texts {
		mailboxes = append(mailboxes, addresses.Mailbox(text))
	}
	return mailboxes
}
