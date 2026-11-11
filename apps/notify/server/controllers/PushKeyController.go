// @RequestRoute("/api/v1/notify/push-key")
// @SignedIn
type PushKeyController struct {
	// @Needs
	pushKeys *PushKeyService
}

// @GetRoute
func (controller *PushKeyController) GetPushKey(
	// @FromHeader("authorization")
	authorization string,
) (PushKey, error) {
	return controller.pushKeys.Key(authorization)
}
