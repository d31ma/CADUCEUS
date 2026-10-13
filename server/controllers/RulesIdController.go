import "net/http"

// @RequestRoute("/rules/{id}")
// @SignedIn
type RulesIdController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	rulesHelper *RulesHelper
}

// @DeleteRoute
func (controller *RulesIdController) DeleteRule(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	rules := RulesRoute{Access: controller.accessHelper, Rules: controller.rulesHelper}
	return rules.Delete(id, request)
}

// @PutRoute
func (controller *RulesIdController) PutRule(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	rules := RulesRoute{Access: controller.accessHelper, Rules: controller.rulesHelper}
	return rules.Update(id, request)
}
