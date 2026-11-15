import "mesail.dev/packages/login"

// @Service
type GrantService struct {
}

func (service *GrantService) Session(grant login.Grant) Session {
	me := Me{MemberId: grant.PersonId, DomainId: grant.DomainId, Username: grant.Username, DisplayName: grant.DisplayName, PrimaryAddress: grant.PrimaryAddress, Roles: grant.Roles}
	return Session{Me: me, Domains: service.domains(grant.Domains), AccessToken: grant.AccessToken, ExpiresIn: grant.ExpiresIn, SessionKey: grant.SessionKey, MustChangePassword: grant.MustChangePassword, Kept: grant.Kept}
}

func (service *GrantService) domains(choices []login.DomainChoice) []DomainChoice {
	domains := []DomainChoice{}
	for _, choice := range choices {
		domains = append(domains, DomainChoice{DomainId: choice.DomainId, Admin: choice.Admin, Default: choice.Default})
	}
	return domains
}
