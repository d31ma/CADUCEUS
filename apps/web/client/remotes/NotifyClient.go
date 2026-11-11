// @HTTPClient("/api/v1/notify")
type NotifyClient interface {
	// @GetRequest("/notifications")
	Notifications(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestParam("cursor")
		cursor BehaviorValue,
		// @RequestParam("limit")
		limit BehaviorValue,
	) (BehaviorValue, error)

	// @PutRequest("/notifications/{notificationId}/read")
	MarkRead(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @PathVariable("notificationId")
		notificationId BehaviorValue,
	) (BehaviorValue, error)

	// @GetRequest("/settings")
	Settings(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
	) (BehaviorValue, error)

	// @PutRequest("/settings")
	SaveSettings(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		settings BehaviorValue,
	) (BehaviorValue, error)
}
