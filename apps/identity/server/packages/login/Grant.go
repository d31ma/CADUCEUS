package login

type Grant struct {
	PersonId           string
	DomainId           string
	Username           string
	DisplayName        string
	PrimaryAddress     string
	Roles              []string
	Domains            []DomainChoice
	AccessToken        string
	ExpiresIn          int64
	SessionKey         string
	MustChangePassword bool
}
