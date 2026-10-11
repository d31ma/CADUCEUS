import "net/http"

// AuthRelay carries one /auth/* request to the authentication boundary and
// reports its answer. It holds no authentication logic: it checks the shape
// of the request, applies the route's rate limit, and passes the boundary's
// result through unchanged.
type AuthRelay struct {
	// Operation is the boundary operation, such as auth.login.begin.
	Operation string
	// Fields are the body fields the operation needs, in the order the
	// route checks them. Each is sent as text.
	Fields []string
	// Signed routes act for the bearer token's session; it is sent as
	// "token" before the fields.
	Signed bool
	// Fixed parameters are sent as they are.
	Fixed map[string]any
	// Limit is the route's rate limit, when it has one, and Limiter counts
	// its attempts.
	Limit   *AuthLimit
	Limiter *RateLimitService
}

// Answer runs the operation for one request.
func (relay AuthRelay) Answer(request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return refusal
	}
	parameters := map[string]any{}
	if relay.Signed {
		token := RouteAccess{}.Bearer(request)
		if token == "" {
			return Answer{}.Unauthorized("Authentication required")
		}
		parameters["token"] = token
	}
	for _, field := range relay.Fields {
		value := BodyText{}.Of(body, field)
		if value == "" {
			return Answer{}.BadRequest(field + " is required")
		}
		parameters[field] = value
	}
	for key, value := range relay.Fixed {
		parameters[key] = value
	}
	if limited := relay.limited(request, parameters); limited != nil {
		return limited
	}
	result, err := AuthClient{Boundary: Boundary{}.FromSettings()}.Call(relay.Operation, parameters)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	return RawAnswer{}.Of(result)
}

func (relay AuthRelay) limited(request *http.Request, parameters map[string]any) *http.Response {
	limit := relay.Limit
	if limit == nil {
		return nil
	}
	key := []string{limit.Name, RecordValues{}.Text(parameters[relay.Fields[0]]), BodyText{}.Address(request)}
	result, err := relay.Limiter.Check(key, limit.Attempts, limit.WindowMinutes*relay.minute())
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	if !result.Allowed {
		return Answer{}.RateLimited("Too many attempts", int(result.RetryAfterSeconds))
	}
	return nil
}

func (AuthRelay) minute() int64 { return 60 * 1000 }

// Session answers the principal the bearer token speaks for, as the boundary
// resolved it.
func (AuthRelay) Session(request *http.Request) *http.Response {
	token := RouteAccess{}.Bearer(request)
	if token == "" {
		return Answer{}.Unauthorized("Authentication required")
	}
	result, err := AuthClient{Boundary: Boundary{}.FromSettings()}.Call("auth.token.authenticate", map[string]any{"token": token})
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	return RawAnswer{}.Of(result)
}
