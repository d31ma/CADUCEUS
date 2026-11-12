import "net/http"

// @RequestRoute("/api/v1/identity")
type IdentityController struct {
	// @Needs
	me *MeService
}

// @GetRoute("/me")
// @SignedIn
func (controller *IdentityController) GetMe(request *http.Request) (Me, error) {
	return controller.me.Me(request)
}
