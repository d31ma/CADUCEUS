// @RequestRoute("/api/v1/realtime/calls/keys")
// @Public
type CallKeysController struct {
	// @Needs
	keys *CallKeysService
}

// @GetRoute
func (controller *CallKeysController) GetKeys() (CallKeys, error) {
	return controller.keys.Keys()
}
