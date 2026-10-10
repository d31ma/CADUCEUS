// SessionService is a minimal local stand-in for the shell's SessionService,
// which the shell area writes in parallel. It keeps the signed-in mailbox in
// sessionStorage and answers the composer's opening event with the session.
//
// @Service
type SessionService struct {
	// @SessionState
	Token string
	// @SessionState
	Email string
}

// CanUseApp says whether a mailbox is signed in.
func (session *SessionService) CanUseApp() bool {
	return session.Token != ""
}

// CanShowLogin says whether the login form shows instead of the app.
func (session *SessionService) CanShowLogin() bool {
	return session.Token == ""
}

// ComposerOpened hands the session to the composer services.
//
// @OnEvent({"replay":true})
func (session *SessionService) ComposerOpened(opened ComposerOpened) SessionChanged {
	return SessionChanged{Token: session.Token, Email: session.Email}
}
