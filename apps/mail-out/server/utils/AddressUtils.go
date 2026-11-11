import (
	"net/mail"
	"strings"
)

type AddressUtils struct{}

func (AddressUtils) Domain(email string) string {
	index := strings.LastIndex(email, "@")
	return strings.ToLower(email[index+1:])
}

func (utils AddressUtils) Plain(text string) string {
	return utils.Mailbox(text).Address
}

func (utils AddressUtils) Mailbox(text string) MailboxAddress {
	address, failure := mail.ParseAddress(text)
	return utils.mailbox(address, failure)
}

func (AddressUtils) mailbox(address *mail.Address, failure error) MailboxAddress {
	if failure != nil {
		return MailboxAddress{}
	}
	return MailboxAddress{Name: address.Name, Address: address.Address}
}
