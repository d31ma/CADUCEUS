// @ErrorHandler
type MailOutErrorHandler struct{}

// @HandlesError
// @ResponseStatus(400)
func (handler *MailOutErrorHandler) Rejected(failure *RejectedEvent) ApiError {
	return ApiError{Code: "invalid_request", Message: failure.Error()}
}

// @HandlesError
// @ResponseStatus(403)
func (handler *MailOutErrorHandler) Forbidden(failure *ForbiddenCaller) ApiError {
	return ApiError{Code: "forbidden", Message: failure.Error()}
}

// @HandlesError
// @ResponseStatus(404)
func (handler *MailOutErrorHandler) Unknown(failure *UnknownDomain) ApiError {
	return ApiError{Code: "not_found", Message: failure.Error()}
}
