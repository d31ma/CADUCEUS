package mailout

import (
	"net/mail"
	"strings"
)

type Addresses struct{}

func (Addresses) Domain(email string) string {
	return strings.ToLower(email[strings.LastIndex(email, "@")+1:])
}

func (addresses Addresses) Mailbox(text string) MailboxAddress {
	return addresses.fromParsed(mail.ParseAddress(text))
}

func (addresses Addresses) Mailboxes(texts []string) []MailboxAddress {
	return addresses.mailboxList(texts)
}

func (Addresses) fromParsed(address *mail.Address, err error) MailboxAddress {
	if err != nil {
		return MailboxAddress{}
	}
	return MailboxAddress{Name: address.Name, Address: address.Address}
}

func (addresses Addresses) mailboxList(texts []string) []MailboxAddress {
	mailboxes := []MailboxAddress{}
	for _, text := range texts {
		mailboxes = append(mailboxes, addresses.Mailbox(text))
	}
	return mailboxes
}
