// One export source: a mailbox domain or a connected account.
type ExportScope struct {
	Domain  string `json:"domain"`
	Account string `json:"account"`
}
