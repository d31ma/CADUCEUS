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

// @HandlesError
// @ResponseStatus(400)
func (handler *IdentityErrorHandler) PasswordPolicy(failure *PasswordPolicy) ApiError {
	return ApiError{Code: "password_policy", Message: failure.Error(), Field: "newPassword"}
}

// @HandlesError
// @ResponseStatus(400)
func (handler *IdentityErrorHandler) InvalidCode(failure *InvalidCode) ApiError {
	return ApiError{Code: "invalid_code", Message: failure.Error(), Field: "code"}
}
