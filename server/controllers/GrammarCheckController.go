import "net/http"

// @RequestRoute("/grammar/check")
// @SignedIn
type GrammarCheckController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	grammarCheckService *GrammarCheckService
	// @Needs
	rateLimitService *RateLimitService
}

// @PostRoute
func (controller *GrammarCheckController) PostCheck(request *http.Request) *http.Response {
	route := GrammarCheckRoute{Access: controller.accessHelper, Grammar: controller.grammarCheckService, RateLimit: controller.rateLimitService}
	return route.Check(request)
}
