type MailSummary struct {
	ID                string `json:"id"`
	Folder            string `json:"folder"`
	SystemFolder      string `json:"systemFolder"`
	Domain            string `json:"domain"`
	ProviderAccountID string `json:"providerAccountId"`
	Read              bool   `json:"read"`
}
