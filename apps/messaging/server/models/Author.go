type Author struct {
	Kind        string `json:"kind"`
	MemberId    string `json:"memberId,omitempty"`
	Address     string `json:"address"`
	DisplayName string `json:"displayName"`
}
