type AddMemberRequest struct {
	Username      string `json:"username"`
	DisplayName   string `json:"displayName,omitempty"`
	FirstPassword string `json:"firstPassword,omitempty"`
	Admin         bool   `json:"admin,omitempty"`
}
