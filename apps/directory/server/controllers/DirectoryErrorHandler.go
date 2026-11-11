// @ErrorHandler
type DirectoryErrorHandler struct {
}

// @HandlesError
// @ResponseStatus(400)
func (handler *DirectoryErrorHandler) Invalid(failure *InvalidRequest) ApiError {
	return ApiError{Code: "invalid_request", Message: failure.Message, Field: failure.Field}
}

// @HandlesError
// @ResponseStatus(401)
func (handler *DirectoryErrorHandler) Unauthenticated(failure *Unauthenticated) ApiError {
	return ApiError{Code: "unauthenticated", Message: failure.Message}
}

// @HandlesError
// @ResponseStatus(403)
func (handler *DirectoryErrorHandler) Forbidden(failure *Forbidden) ApiError {
	return ApiError{Code: handler.forbidden(), Message: failure.Message}
}

// @HandlesError
// @ResponseStatus(400)
func (handler *DirectoryErrorHandler) PasswordPolicy(failure *PasswordPolicy) ApiError {
	return ApiError{Code: "password_policy", Message: failure.Message, Field: "password"}
}

// @HandlesError
// @ResponseStatus(409)
func (handler *DirectoryErrorHandler) LastAdmin(failure *LastAdmin) ApiError {
	return ApiError{Code: "last_admin", Message: failure.Message}
}

// @HandlesError
// @ResponseStatus(404)
func (handler *DirectoryErrorHandler) Missing(failure *NotFound) ApiError {
	return ApiError{Code: "not_found", Message: failure.Message}
}

// @HandlesError
// @ResponseStatus(404)
func (handler *DirectoryErrorHandler) NotHosted(failure *DomainNotHosted) ApiError {
	return ApiError{Code: "domain_not_hosted", Message: failure.Message}
}

// @HandlesError
// @ResponseStatus(409)
func (handler *DirectoryErrorHandler) Duplicate(failure *Duplicate) ApiError {
	return ApiError{Code: "duplicate", Message: failure.Message}
}

// @HandlesError
// @ResponseStatus(503)
func (handler *DirectoryErrorHandler) Unavailable(failure *Unavailable) ApiError {
	return ApiError{Code: handler.unavailable(), Message: failure.Message}
}

// @HandlesError
// @ResponseStatus(503)
func (handler *DirectoryErrorHandler) Store(failure YonResourceError) ApiError {
	return ApiError{Code: handler.unavailable(), Message: "the directory store is not available"}
}

func (handler *DirectoryErrorHandler) forbidden() string {
	return "forbidden"
}

func (handler *DirectoryErrorHandler) unavailable() string {
	return "unavailable"
}
