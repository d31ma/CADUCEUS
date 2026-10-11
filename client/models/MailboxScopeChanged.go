// Which mailbox and folder the list shows, for the search box and the archive tools.
type MailboxScopeChanged struct {
	Folder       string   `json:"folder"`
	FolderTitle  string   `json:"folderTitle"`
	StatusFilter string   `json:"statusFilter"`
	Domain       string   `json:"domain"`
	Account      string   `json:"account"`
	AccountLabel string   `json:"accountLabel"`
	DomainLabel  string   `json:"domainLabel"`
	Owner        string   `json:"owner"`
	Domains      []string `json:"domains"`
	Total        int      `json:"total"`
	Searching    bool     `json:"searching"`
}
