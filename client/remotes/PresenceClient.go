// PresenceClient is the /presence route group: the shell's heartbeat and the
// team roster.
// @HTTPClient("/presence")
type PresenceClient interface {
	// @PostRequest("/heartbeat")
	Heartbeat(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
	) (BehaviorValue, error)

	// @GetRequest("/team")
	Team(
		// @RequestHeader("Authorization")
		authorization string,
	) ([]Teammate, error)
}
