import (
	"encoding/json"
	"strings"
)

// SessionService owns who is signed in. The legacy caduceus_* sessionStorage
// keys stay the source of truth: BrowserStorageService hands them over in
// StoredValues, and every change goes back as StorageWritten. Other services
// cannot inject this one (each Go service is its own program), so they hear
// the retained SessionChanged event instead and ask for a sign-out with
// SignOutRequested or for a token check with SessionCheckRequested.
// @Service
type SessionService struct {
	// @Needs
	authClient                *AuthClient
	applicationEventPublisher ApplicationEventPublisher
	SignedIn                  bool
	Restored                  bool
	Email                     string
	Name                      string
	DisplayName               string
	Role                      string
	Domains                   []string
	token                     string
	refreshToken              string
	idToken                   string
}

// Restore adopts the session that this tab already holds.
// @OnEvent({"replay":true})
func (session *SessionService) Restore(stored StoredValues) {
	values := stored.Session
	session.token = values[session.keys().Token]
	session.refreshToken = values[session.keys().Refresh]
	session.idToken = values[session.keys().IDToken]
	session.Email = values[session.keys().Email]
	session.Name = values[session.keys().Name]
	session.Role = values[session.keys().Role]
	session.Domains = session.domainsFrom(values[session.keys().Domains])
	session.Restored = true
	session.announce()
}

// HandleLogin stores the token set that the login form received and opens
// the inbox, like the shell's handleLogin did.
// @OnEvent
func (session *SessionService) HandleLogin(tokens TokenSet) {
	token := TextUtils{}.FirstOf(tokens.AccessToken, tokens.Token)
	address := TextUtils{}.FirstOf(tokens.Address, tokens.Email)
	session.loginOutcome(session.issued(token, address), tokens)(token, address)
}

// SignOut ends the session on the server, forgets it in this tab and returns
// to the landing page.
func (session *SessionService) SignOut() {
	session.endOnServer(session.idToken)
	session.forget()
	session.announce()
	session.applicationEventPublisher.PublishEvent(NavigationRequested{Target: "/", Replace: false}, false)
}

// RequestSignOut lets another service end the session.
// @OnEvent
func (session *SessionService) RequestSignOut(request SignOutRequested) {
	session.SignOut()
}

// Check asks the server whether the stored token still speaks for a live
// session, and signs out when it does not. Other services publish
// SessionCheckRequested after an authenticated call fails, because the
// browser transport reports every error status the same way.
// @OnEvent
func (session *SessionService) Check(request SessionCheckRequested) {
	_, failure := session.authClient.Session(session.bearer())
	session.afterCheck(HTTPUtils{}.Refused(failure))
}

// Refresh rotates the refresh token for a new access token. The original
// shell stored the refresh token but never used it; another service asks
// for a refresh with SessionRefreshRequested and hears the new token in
// SessionChanged.
// @OnEvent
func (session *SessionService) Refresh(request SessionRefreshRequested) {
	tokens, failure := session.authClient.Refresh(RefreshRequest{RefreshToken: session.refreshToken, ClientID: request.ClientID})
	session.afterRefresh(tokens, failure)
}

func (session *SessionService) bearer() string {
	return "Bearer " + session.token
}

func (session *SessionService) announce() {
	session.SignedIn = session.token != ""
	session.DisplayName = NameUtils{}.DisplayName(session.Name, session.Email)
	session.applicationEventPublisher.PublishEvent(SessionChanged{
		SignedIn:     session.SignedIn,
		Token:        session.token,
		Email:        session.Email,
		Name:         session.Name,
		DisplayName:  session.DisplayName,
		Role:         session.Role,
		Domains:      session.Domains,
		RefreshToken: session.refreshToken,
	}, true)
}

func (session *SessionService) loginOutcome(issued bool, tokens TokenSet) func(string, string) {
	if !issued {
		return session.rejectLogin
	}
	return func(token string, address string) { session.storeLogin(tokens, token, address) }
}

func (session *SessionService) rejectLogin(token string, address string) {
	session.applicationEventPublisher.PublishEvent(ToastRequested{Message: "Login failed: invalid response", DurationMs: 2500}, false)
}

func (session *SessionService) storeLogin(tokens TokenSet, token string, address string) {
	session.token = token
	session.refreshToken = tokens.RefreshToken
	session.idToken = tokens.IDToken
	session.Email = address
	session.Name = tokens.Name
	session.Role = tokens.Role
	session.Domains = session.loginDomains(tokens.Domain, tokens.Domains)
	session.write(session.storedLogin(), session.loginRemovals())
	session.announce()
	session.applicationEventPublisher.PublishEvent(NavigationRequested{Target: "/inbox", Replace: true}, false)
}

func (session *SessionService) storedLogin() map[string]string {
	encoded, _ := json.Marshal(session.Domains)
	return map[string]string{
		session.keys().Token:     session.token,
		session.keys().Email:     session.Email,
		session.keys().Name:      session.Name,
		session.keys().Role:      session.Role,
		session.keys().Domains:   string(encoded),
		session.keys().Refresh:   session.refreshToken,
		session.keys().IDToken:   session.idToken,
	}
}

func (session *SessionService) loginRemovals() []string {
	return []string{session.keys().MailboxNames, session.keys().InterfaceFont, session.keys().ComposerFont, session.keys().Sounds}
}

func (session *SessionService) forget() {
	session.token = ""
	session.refreshToken = ""
	session.idToken = ""
	session.Email = ""
	session.Name = ""
	session.Role = ""
	session.Domains = []string{}
	session.write(map[string]string{}, []string{
		session.keys().Token, session.keys().Email, session.keys().Name, session.keys().Role, session.keys().Domains,
		session.keys().MailboxNames, session.keys().Refresh, session.keys().IDToken,
		session.keys().InterfaceFont, session.keys().ComposerFont, session.keys().Sounds, session.keys().Mail,
	})
}

func (session *SessionService) write(values map[string]string, removed []string) {
	session.applicationEventPublisher.PublishEvent(StorageWritten{Area: "session", Values: values, Removed: removed}, false)
}

func (session *SessionService) endOnServer(idToken string) {
	if idToken == "" {
		return
	}
	session.authClient.Logout(LogoutRequest{IDToken: idToken})
}

func (session *SessionService) afterCheck(refused bool) {
	if refused {
		session.SignOut()
	}
}

func (session *SessionService) afterRefresh(tokens TokenSet, failure error) {
	if failure != nil {
		return
	}
	session.storeRefresh(tokens)
}

func (session *SessionService) storeRefresh(tokens TokenSet) {
	session.token = TextUtils{}.FirstOf(tokens.AccessToken, tokens.Token)
	session.refreshToken = TextUtils{}.FirstOf(tokens.RefreshToken, session.refreshToken)
	session.write(map[string]string{session.keys().Token: session.token, session.keys().Refresh: session.refreshToken}, []string{})
	session.announce()
}

func (session *SessionService) issued(token string, address string) bool {
	return token != "" && address != ""
}


func (session *SessionService) loginDomains(domain string, domains []string) []string {
	if domain != "" {
		return []string{domain}
	}
	return append([]string{}, domains...)
}

func (session *SessionService) domainsFrom(text string) []string {
	domains := []string{}
	if json.Unmarshal([]byte(strings.TrimSpace(text)), &domains) != nil {
		return []string{}
	}
	return domains
}

// keys are the legacy sessionStorage names that the e2e suites and the
// unported pages still read.
func (SessionService) keys() SessionKeys {
	return SessionKeys{
		Token: "caduceus_token", Email: "caduceus_email", Name: "caduceus_name", Role: "caduceus_role",
		Domains: "caduceus_domains", Refresh: "caduceus_refresh", IDToken: "caduceus_id_token",
		MailboxNames: "caduceus_mailbox_names", InterfaceFont: "caduceus_interface_font",
		ComposerFont: "caduceus_composer_font", Sounds: "caduceus_notification_sounds", Mail: session.keys().Mail,
	}
}
