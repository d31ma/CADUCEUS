type CategoryAccount struct {
	Domain     string            `json:"domain"`
	Categories []MailCategory    `json:"categories"`
	Mappings   map[string]string `json:"mappings"`
}
