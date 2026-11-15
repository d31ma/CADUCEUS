import "mesail.dev/packages/apierrors"

// @ErrorHandler
type DirectoryErrorHandler struct {
}

// @HandlesError
// @ResponseStatus(400)
func (handler *DirectoryErrorHandler) Invalid(failure *InvalidRequest) ApiError {
	return handler.answer(failure.ErrorCase)
}

// @HandlesError
// @ResponseStatus(401)
func (handler *DirectoryErrorHandler) Unauthenticated(failure *Unauthenticated) ApiError {
	return handler.answer(failure.ErrorCase)
}

// @HandlesError
// @ResponseStatus(403)
func (handler *DirectoryErrorHandler) Forbidden(failure *Forbidden) ApiError {
	return handler.answer(failure.ErrorCase)
}

// @HandlesError
// @ResponseStatus(503)
func (handler *DirectoryErrorHandler) Store(failure YonResourceError) ApiError {
	return handler.Unavailable(Unavailable{}.Store())
}

// @HandlesError
// @ResponseStatus(400)
func (handler *DirectoryErrorHandler) PasswordPolicy(failure *PasswordPolicy) ApiError {
	return handler.answer(failure.ErrorCase)
}

// @HandlesError
// @ResponseStatus(409)
func (handler *DirectoryErrorHandler) LastAdmin(failure *LastAdmin) ApiError {
	return handler.answer(failure.ErrorCase)
}

// @HandlesError
// @ResponseStatus(404)
func (handler *DirectoryErrorHandler) Missing(failure *NotFound) ApiError {
	return handler.answer(failure.ErrorCase)
}

// @HandlesError
// @ResponseStatus(500)
func (handler *DirectoryErrorHandler) Unexpected(failure error) ApiError {
	return handler.answer(UnexpectedFailure{}.Unexpected().ErrorCase)
}

// @HandlesError
// @ResponseStatus(404)
func (handler *DirectoryErrorHandler) NotHosted(failure *DomainNotHosted) ApiError {
	return handler.answer(failure.ErrorCase)
}

// @HandlesError
// @ResponseStatus(409)
func (handler *DirectoryErrorHandler) Duplicate(failure *Duplicate) ApiError {
	return handler.answer(failure.ErrorCase)
}

// @HandlesError
// @ResponseStatus(503)
func (handler *DirectoryErrorHandler) Unavailable(failure *Unavailable) ApiError {
	return handler.answer(failure.ErrorCase)
}

func (handler *DirectoryErrorHandler) answer(failure ErrorCase) ApiError {
	return ApiError{Code: failure.Code(), Message: apierrors.Message{}.Format(failure.Number(), failure.Error()), Field: failure.Field()}
}
