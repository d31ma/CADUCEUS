import "mesail.dev/packages/apierrors"

// @ErrorHandler
type IdentityErrorHandler struct{}

// @HandlesError
// @ResponseStatus(400)
func (handler *IdentityErrorHandler) BadRequest(failure *BadRequest) ApiError {
	return handler.answer(failure.ErrorCase)
}

// @HandlesError
// @ResponseStatus(401)
func (handler *IdentityErrorHandler) Unauthenticated(failure *Unauthenticated) ApiError {
	return handler.answer(failure.ErrorCase)
}

// @HandlesError
// @ResponseStatus(403)
func (handler *IdentityErrorHandler) Forbidden(failure *Forbidden) ApiError {
	return handler.answer(failure.ErrorCase)
}

// @HandlesError
// @ResponseStatus(404)
func (handler *IdentityErrorHandler) NotFound(failure *NotFound) ApiError {
	return handler.answer(failure.ErrorCase)
}

// @HandlesError
// @ResponseStatus(409)
func (handler *IdentityErrorHandler) Conflict(failure *Conflict) ApiError {
	return handler.answer(failure.ErrorCase)
}

// @HandlesError
// @ResponseStatus(429)
func (handler *IdentityErrorHandler) RateLimited(failure *RateLimited) ApiError {
	return handler.answer(failure.ErrorCase)
}

// @HandlesError
// @ResponseStatus(503)
func (handler *IdentityErrorHandler) Unavailable(failure *Unavailable) ApiError {
	return handler.answer(failure.ErrorCase)
}

// @HandlesError
// @ResponseStatus(500)
func (handler *IdentityErrorHandler) Unexpected(failure error) ApiError {
	return handler.answer(UnexpectedFailure{}.Unexpected().ErrorCase)
}

func (handler *IdentityErrorHandler) answer(failure ErrorCase) ApiError {
	return ApiError{Code: failure.Code(), Message: apierrors.Message{}.Format(failure.Number(), failure.Error()), Field: failure.Field()}
}
