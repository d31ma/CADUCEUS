// SendClient is the /send route group. The shell undoes a send that the
// composer left pending across a navigation.
// @HTTPClient("/send")
type SendClient interface {
	// @PostRequest("/undo")
	Undo(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		undo BehaviorValue,
	) (BehaviorValue, error)
}
