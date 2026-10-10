import "net/http"

// @RequestRoute("/rules/order")
// @SignedIn
type RulesOrderController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	rulesHelper *RulesHelper
}

// @PutRoute
func (controller *RulesOrderController) PutOrder(request *http.Request) *http.Response {
	return RulesRoute{Access: controller.accessHelper, Rules: controller.rulesHelper}.Reorder(request)
}
