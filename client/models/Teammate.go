type Teammate struct {
	Email      string `json:"email"`
	Name       string `json:"name"`
	Online     bool   `json:"online"`
	IsSelf     bool   `json:"isSelf"`
	LastSeenAt string `json:"lastSeenAt"`
}
