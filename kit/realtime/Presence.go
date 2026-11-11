package realtime

type Presence struct {
	MemberId          string
	State             string
	DoNotDisturbUntil string
	LastSeenAt        string
}
