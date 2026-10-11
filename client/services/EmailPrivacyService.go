// EmailPrivacyService holds the remote image policy, the senders whose
// images may load, and the opt-in public GitHub profile lookup.
// @Service
type EmailPrivacyService struct {
	// @Needs
	usersClient               *UsersClient
	applicationEventPublisher ApplicationEventPublisher
	authorization             string
	RemoteImagePolicy         string
	TrustedSenders            []string
	GithubProfiles            bool
	Saving                    bool
	Error                     string
}

// @OnStart
func (service *EmailPrivacyService) Start() {
	service.apply(UserPreferences{})
}

// @OnEvent({"replay":true})
func (service *EmailPrivacyService) HearSession(changed SessionChanged) {
	service.authorization = "Bearer " + changed.Token
}

// @OnEvent({"replay":true})
func (service *EmailPrivacyService) Load(preferences UserPreferences) {
	service.apply(preferences)
}

func (service *EmailPrivacyService) SetRemoteImagePolicy(policy string) error {
	service.RemoteImagePolicy = policy
	service.save(map[string]any{"remoteImagePolicy": policy})
	return nil
}

func (service *EmailPrivacyService) RemoveTrustedSender(sender string) error {
	service.save(map[string]any{"trustedImageSenders": PreferenceUtils{}.Without(service.TrustedSenders, sender)})
	return nil
}

func (service *EmailPrivacyService) SetGithubProfiles(enabled bool) error {
	service.GithubProfiles = enabled
	service.save(map[string]any{"githubProfileEnrichment": enabled})
	return nil
}

func (service *EmailPrivacyService) save(patch map[string]any) {
	FlowUtils{}.When(!service.Saving, func() { service.send(patch) })()
}

func (service *EmailPrivacyService) send(patch map[string]any) {
	service.Saving = true
	service.Error = ""
	stored, failure := service.usersClient.SavePreferences(service.authorization, patch)
	service.Saving = false
	FlowUtils{}.Either(failure == nil, func() { service.showSaved(stored) }, func() {
		service.Error = HTTPUtils{}.Message(failure, "Could not save remote-content preferences.", "Network error.")
	})()
}

func (service *EmailPrivacyService) showSaved(stored UserPreferences) {
	service.apply(stored)
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: "Privacy preference saved."}, false)
}

func (service *EmailPrivacyService) apply(preferences UserPreferences) {
	allow := "allow"
	service.RemoteImagePolicy = ChoiceUtils{}.Text(preferences.RemoteImagePolicy == allow, allow, "block")
	service.TrustedSenders = append([]string{}, preferences.TrustedImageSenders...)
	service.GithubProfiles = preferences.GithubProfileEnrichment
}
