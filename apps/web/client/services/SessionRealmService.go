import (
	"net/url"
	"regexp"
	"strings"
)

// @Service
type SessionRealmService struct {
	config ClientConfig
	// @Needs
	identity *IdentityClient
}

func (realm *SessionRealmService) Load() error {
	config, failure := BehaviorsAnswer[ClientConfig](realm.identity.Config())
	realm.config = config
	return failure
}

func (realm *SessionRealmService) DomainOf(email string) string {
	address := strings.ToLower(strings.TrimSpace(email))
	return realm.domainWhenValid(address, realm.emailPattern().MatchString(address))
}

func (realm *SessionRealmService) Issuer(domainId string) string {
	base, _, _ := strings.Cut(realm.config.Issuer, realm.realmsSegment())
	return strings.TrimSuffix(base, "/") + realm.realmsSegment() + url.PathEscape(domainId)
}

func (realm *SessionRealmService) SignInHost(domainId string) string {
	parsed, failure := url.Parse(realm.Issuer(domainId))
	return realm.hostWhenParsed(parsed, failure)
}

func (realm *SessionRealmService) RedirectUri() string {
	return realm.config.RedirectUri
}

func (realm *SessionRealmService) AuthorizeUrl(pending SessionPending, challenge string) string {
	query := url.Values{}
	query.Set("response_type", "code")
	query.Set(realm.clientIdKey(), realm.config.ClientId)
	query.Set("redirect_uri", realm.config.RedirectUri)
	query.Set("scope", strings.Join(realm.config.Scopes, " "))
	query.Set("state", pending.State)
	query.Set("code_challenge", challenge)
	query.Set("code_challenge_method", "S256")
	query.Set("login_hint", pending.Email)
	return realm.Issuer(pending.DomainId) + "/protocol/openid-connect/auth?" + query.Encode()
}

func (realm *SessionRealmService) LogoutUrl(domainId string, idToken string) string {
	query := url.Values{}
	query.Set(realm.clientIdKey(), realm.config.ClientId)
	query.Set("post_logout_redirect_uri", realm.config.RedirectUri+realm.signedOutPath())
	realm.addHint(query, idToken)
	return realm.Issuer(domainId) + "/protocol/openid-connect/logout?" + query.Encode()
}

func (realm *SessionRealmService) domainWhenValid(address string, valid bool) string {
	if !valid {
		return ""
	}
	return address[strings.LastIndex(address, "@")+1:]
}

func (realm *SessionRealmService) hostWhenParsed(parsed *url.URL, failure error) string {
	if failure != nil {
		return ""
	}
	return parsed.Host
}

func (realm *SessionRealmService) addHint(query url.Values, idToken string) {
	if idToken == "" {
		return
	}
	query.Set("id_token_hint", idToken)
}

func (realm *SessionRealmService) emailPattern() *regexp.Regexp {
	return regexp.MustCompile(`^[^@\s]+@([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+[a-z]{2,}$`)
}

func (realm *SessionRealmService) realmsSegment() string {
	return "/realms/"
}

func (realm *SessionRealmService) clientIdKey() string {
	return "client_id"
}

func (realm *SessionRealmService) signedOutPath() string {
	return "signed-out"
}
