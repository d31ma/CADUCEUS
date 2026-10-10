// SessionService is a minimal stand-in, under the same name, for the shell
// area's SessionService (written in parallel). It adopts the legacy
// caduceus_token and caduceus_email sessionStorage keys that
// BrowserStorageService hands over and announces them in the retained
// SessionChanged event, which CalendarService and ContactsService hear.
// @Service
type SessionService struct {
	applicationEventPublisher ApplicationEventPublisher
	SignedIn                  bool
	Restored                  bool
	token                     string
	email                     string
}

// Restore adopts the session that this tab already holds.
// @OnEvent({"replay":true})
func (session *SessionService) Restore(stored StoredValues) {
	session.token = stored.Session["caduceus_token"]
	session.email = stored.Session["caduceus_email"]
	session.SignedIn = session.token != ""
	session.Restored = true
	session.applicationEventPublisher.PublishEvent(SessionChanged{SignedIn: session.SignedIn, Token: session.token, Email: session.email}, true)
}
