// NotificationScope is what a user may be notified about: the domains
// assigned to the user and the ids of the user's connected accounts.
type NotificationScope struct {
	Domains  any
	Accounts []string
}
