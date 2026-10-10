import (
	"fmt"
	"net/http"
)

// GrammarCheckRoute answers /grammar/check: one bounded, authenticated
// grammar check.
type GrammarCheckRoute struct {
	Access    *AccessHelper
	Grammar   *GrammarCheckService
	RateLimit *RateLimitService
}

// Check validates the text, language and disabled rules, limits each
// mailbox to 120 checks a minute, and answers the issues.
func (route GrammarCheckRoute) Check(request *http.Request) *http.Response {
	value, refusal := RequestBody{}.Value(request)
	if refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	body, _ := value.(map[string]any)
	text, isText := body["text"].(string)
	if !isText {
		return Answer{}.BadRequest("text must be a string")
	}
	check := GrammarCheck{}
	if (TextValue{}).Length(text) > check.MaximumTextLength() {
		return Answer{}.BadRequest(fmt.Sprintf("text may contain at most %d characters", check.MaximumTextLength()))
	}
	language := RecordValues{}.Coalesce(body["language"], "auto")
	if !check.IsLanguage(language) {
		return Answer{}.BadRequest("language must be one of: auto, en-US, en-GB")
	}
	disabledRules := []string{}
	if rules, present := body["disabledRules"]; present {
		normalized, valid := check.DisabledRules(rules)
		if !valid {
			return Answer{}.BadRequest("disabledRules must contain at most 100 valid rule identifiers")
		}
		disabledRules = normalized
	}
	limit, err := route.RateLimit.Check([]string{"grammar-check", caller.Email}, 120, 60_000)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	if !limit.Allowed {
		return Answer{}.RateLimited("Grammar check rate limit reached", int(limit.RetryAfterSeconds))
	}
	return Answer{}.Ok(route.Grammar.Check(text, language.(string), disabledRules))
}
