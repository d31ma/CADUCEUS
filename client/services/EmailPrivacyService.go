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
	service.apply(map[string]any{})
}

// @OnEvent({"replay":true})
func (service *EmailPrivacyService) Open(session AccountSession) {
	service.authorization = "Bearer " + session.Token
}

// @OnEvent({"replay":true})
func (service *EmailPrivacyService) Load(preferences UserPreferences) {
	service.apply(preferences.Values)
}

func (service *EmailPrivacyService) SetRemoteImagePolicy(policy string) error {
	service.RemoteImagePolicy = policy
	service.save(map[string]any{service.policyKey(): policy})
	return nil
}

func (service *EmailPrivacyService) RemoveTrustedSender(sender string) error {
	service.save(map[string]any{service.sendersKey(): PreferenceUtils{}.Without(service.TrustedSenders, sender)})
	return nil
}

func (service *EmailPrivacyService) SetGithubProfiles(enabled bool) error {
	service.GithubProfiles = enabled
	service.save(map[string]any{service.githubKey(): enabled})
	return nil
}

func (service *EmailPrivacyService) save(patch map[string]any) {
	FlowUtils{}.When(!service.Saving, func() { service.send(patch) })
}

func (service *EmailPrivacyService) send(patch map[string]any) {
	service.Saving = true
	service.Error = ""
	stored, failure := service.usersClient.SavePreferences(service.authorization, patch)
	service.Saving = false
	service.showSaved(PreferenceUtils{}.Object(stored), failure)
}

func (service *EmailPrivacyService) showSaved(stored map[string]any, failure error) {
	if failure != nil {
		service.Error = "Could not save remote-content preferences."
		return
	}
	service.apply(stored)
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: "Privacy preference saved."}, false)
}

func (service *EmailPrivacyService) apply(values map[string]any) {
	allow := "allow"
	service.RemoteImagePolicy = FlowUtils{}.Text(PreferenceUtils{}.Text(values, service.policyKey()) == allow, allow, "block")
	service.TrustedSenders = PreferenceUtils{}.Strings(values, service.sendersKey())
	service.GithubProfiles = PreferenceUtils{}.Flag(values, service.githubKey())
}

func (service *EmailPrivacyService) policyKey() string { return "remoteImagePolicy" }

func (service *EmailPrivacyService) sendersKey() string { return "trustedImageSenders" }

func (service *EmailPrivacyService) githubKey() string { return "githubProfileEnrichment" }
