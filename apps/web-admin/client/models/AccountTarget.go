type AccountTarget struct {
	PersonId       string `json:"personId"`
	Name           string `json:"name"`
	Username       string `json:"username"`
	Address        string `json:"address"`
	Admin          bool   `json:"admin"`
	AccountManaged bool   `json:"accountManaged"`
	Disabled       bool   `json:"disabled"`
	Member         bool   `json:"member"`
}
