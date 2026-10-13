type GithubRepository struct {
	FullName    string `json:"fullName"`
	Url         string `json:"url"`
	Stars       int    `json:"stars"`
	Description string `json:"description"`
}
