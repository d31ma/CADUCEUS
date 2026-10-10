// GitHubProfilesService looks up the public GitHub profile of an address
// for mailboxes that opted in.
// @Service
type GitHubProfilesService struct {
	cache GitHubProfileCache
}

// FindByEmail answers the found profile, not-found or unavailable.
func (service *GitHubProfilesService) FindByEmail(email string) map[string]any {
	return GitHubProfiles{Cache: &service.cache}.FindByEmail(email)
}
