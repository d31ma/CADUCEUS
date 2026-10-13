type GithubProfile struct {
	Login        string             `json:"login"`
	ProfileUrl   string             `json:"profileUrl"`
	Repositories []GithubRepository `json:"repositories"`
}
