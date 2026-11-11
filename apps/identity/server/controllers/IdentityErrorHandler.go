// @ErrorHandler
type IdentityErrorHandler struct{}

// @HandlesError
// @ResponseStatus(401)
func (handler *IdentityErrorHandler) Unauthenticated(failure *Unauthenticated) ApiError {
	return ApiError{Code: "unauthenticated", Message: failure.Error()}
}

// @HandlesError
// @ResponseStatus(429)
func (handler *IdentityErrorHandler) RateLimited(failure *RateLimited) ApiError {
	return ApiError{Code: "rate_limited", Message: failure.Error()}
}

// @HandlesError
// @ResponseStatus(503)
func (handler *IdentityErrorHandler) Unavailable(failure *Unavailable) ApiError {
	return ApiError{Code: "unavailable", Message: failure.Error()}
}
