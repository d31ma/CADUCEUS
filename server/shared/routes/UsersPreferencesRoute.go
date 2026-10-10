import "net/http"

// UsersPreferencesRoute answers /users/me/preferences: the caller's display
// and reading preferences. Mailbox names are display-only labels for domains
// already assigned to the caller; authorization still uses the domains.
type UsersPreferencesRoute struct {
	Access *AccessHelper
	Users  *UsersHelper
}

// Read answers the caller's preferences.
func (route UsersPreferencesRoute) Read(request *http.Request) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	_, user, err := route.Users.FindByEmail(caller.Email)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	if user == nil {
		return Answer{}.NotFound("User not found")
	}
	notifications := NewMailNotificationPreferences{}.Normalized(user, user["domains"])
	return Answer{}.Ok(route.view(user, notifications))
}

// Update checks every preference the body names, then stores them together:
// a refused update changes nothing.
func (route UsersPreferencesRoute) Update(request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	documentId, user, err := route.Users.FindByEmail(caller.Email)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	if documentId == "" || user == nil {
		return Answer{}.NotFound("User not found")
	}
	next := StoredRecords{}.Without(user)
	if problem := (PreferencesUpdate{Body: body, User: user, Next: next}).Applied(); problem != nil {
		return problem.Answer()
	}
	accounts, refusal := route.accounts(body, user, caller)
	if refusal != nil {
		return refusal
	}
	if problem := route.notifications(body, user, next, accounts); problem != nil {
		return problem.Answer()
	}
	if raw, named := body["mailPreferences"]; named {
		patch := MailPreferencesPatch{Email: RecordValues{}.Text(user["email"]), Aliases: TextList{}.Of(user["aliases"]), Domains: TextList{}.Of(user["domains"]), Current: user["mailPreferences"]}
		preferences, problem := patch.Checked(raw)
		if problem != nil {
			return problem.Answer()
		}
		next["mailPreferences"] = preferences
	}
	updated, err := route.Users.UpdatePreferences(documentId, next)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	view := route.view(updated, NewMailNotificationPreferences{}.ForAccounts(updated, updated["domains"], accounts))
	view["updated"] = true
	return Answer{}.Ok(view)
}

// accounts answers the connected accounts that may notify: the stored list,
// or the caller's current accounts when the body names a new choice.
func (UsersPreferencesRoute) accounts(body map[string]any, user map[string]any, caller RouteCaller) ([]string, *http.Response) {
	if _, named := body["newMailNotificationExternalAccounts"]; !named {
		return TextList{}.Of(user["newMailNotificationExternalAccounts"]), nil
	}
	return ExternalAccountIds{}.Of(caller.Token)
}

func (UsersPreferencesRoute) notifications(body map[string]any, user map[string]any, next map[string]any, accounts []string) *PreferenceProblem {
	scope := NotificationScope{Domains: user["domains"], Accounts: accounts}
	preferences, problem := NewMailNotificationPreferences{}.Checked(body, user, scope)
	if problem != nil {
		return problem
	}
	for _, field := range []string{"newMailNotificationDomains", "newMailNotificationPrivacy", "newMailNotificationActions"} {
		next[field] = preferences[field]
	}
	_, named := body["newMailNotificationExternalAccounts"]
	if _, isList := (RecordValues{}).List(user["newMailNotificationExternalAccounts"]); named || isList {
		next["newMailNotificationExternalAccounts"] = preferences["newMailNotificationExternalAccounts"]
	}
	return nil
}

// view answers the preferences of a stored user with every default filled in.
func (UsersPreferencesRoute) view(user map[string]any, notifications map[string]any) map[string]any {
	values := RecordValues{}
	domains := user["domains"]
	view := map[string]any{
		"calendarVisibility":        values.Or(user["calendarVisibility"], "private"),
		"mailboxNames":              values.Or(user["mailboxNames"], map[string]any{}),
		"categoryMappings":          values.Or(user["categoryMappings"], map[string]any{}),
		"remoteImagePolicy":         values.Or(user["remoteImagePolicy"], "block"),
		"trustedImageSenders":       values.Or(user["trustedImageSenders"], []any{}),
		"interfaceFont":             values.Or(user["interfaceFont"], FontPreferences{}.InterfaceFont(nil)),
		"composerFont":              values.Or(user["composerFont"], FontPreferences{}.ComposerFont(nil)),
		"interfaceLocale":           values.Or(user["interfaceLocale"], LocalePreferences{}.InterfaceLocale(nil)),
		"notificationSounds":        values.Or(user["notificationSounds"], NotificationSounds{}.Normalized(nil)),
		"themePreferences":          values.Or(user["themePreferences"], ThemePreferences{}.Normalized(nil)),
		"githubProfileEnrichment":   user["githubProfileEnrichment"] == true,
		"keyboardShortcutOverrides": KeyboardShortcuts{}.Overrides(user["keyboardShortcutOverrides"]),
		"mailPreferences":           MailPreferences{}.Normalized(user["mailPreferences"], RecordValues{}.Text(user["email"]), TextList{}.Of(user["aliases"]), TextList{}.Of(domains)),
	}
	for _, group := range []map[string]any{(UnreadBadgePreferences{}).Normalized(user, domains), notifications, (GrammarPreferences{}).Normalized(user)} {
		for key, value := range group {
			view[key] = value
		}
	}
	return view
}
