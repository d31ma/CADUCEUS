// AddressBook is a writable CardDAV address book of a connected account, a
// destination for a new contact.
type AddressBook struct {
	AccountID      string `json:"accountId"`
	AddressBookID  string `json:"addressBookId"`
	Name           string `json:"name"`
	AccountLabel   string `json:"accountLabel"`
	MailboxAddress string `json:"mailboxAddress"`
}
