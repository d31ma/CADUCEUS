// Where the page was opened: the folder, domain and account query of its address.
type MailboxOpened struct {
	Folder  string `json:"folder"`
	Domain  string `json:"domain"`
	Account string `json:"account"`
}
