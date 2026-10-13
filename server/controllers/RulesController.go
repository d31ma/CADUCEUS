import "net/http"

// @RequestRoute("/rules")
// @SignedIn
type RulesController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	rulesHelper *RulesHelper
	// @Needs
	usersHelper *UsersHelper
}

// @PostRoute
func (controller *RulesController) PostRule(request *http.Request) *http.Response {
	rules := RulesRoute{Access: controller.accessHelper, Rules: controller.rulesHelper, Users: controller.usersHelper}
	return rules.Create(request)
}

// @GetRoute
func (controller *RulesController) GetRules(request *http.Request) *http.Response {
	return RulesRoute{Access: controller.accessHelper, Rules: controller.rulesHelper}.List(request)
}
