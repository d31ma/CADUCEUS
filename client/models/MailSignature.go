type MailSignature struct {
	ID         string   `json:"id"`
	Domain     string   `json:"domain"`
	Name       string   `json:"name"`
	Text       string   `json:"text"`
	DefaultFor []string `json:"defaultFor"`
	CreatedAt  string   `json:"createdAt"`
	UpdatedAt  string   `json:"updatedAt"`
}
