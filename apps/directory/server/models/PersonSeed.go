type PersonSeed struct {
	PersonId    string           `json:"personId"`
	Username    string           `json:"username"`
	DisplayName string           `json:"displayName"`
	Memberships []SeedMembership `json:"memberships"`
}
