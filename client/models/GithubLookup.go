type GithubLookup struct {
	Status  string         `json:"status"`
	Profile *GithubProfile `json:"profile"`
}
