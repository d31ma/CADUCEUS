// @ErrorHandler
type MessagingErrorHandler struct {
}

// @HandlesError
// @ResponseStatus(400)
func (handler *MessagingErrorHandler) Invalid(failure *InvalidRequest) ApiError {
	return ApiError{Code: "invalid_request", Message: failure.Message, Field: failure.Field}
}

// @HandlesError
// @ResponseStatus(401)
func (handler *MessagingErrorHandler) Unauthenticated(failure *Unauthenticated) ApiError {
	return ApiError{Code: "unauthenticated", Message: failure.Message}
}

// @HandlesError
// @ResponseStatus(403)
func (handler *MessagingErrorHandler) Forbidden(failure *Forbidden) ApiError {
	return ApiError{Code: "forbidden", Message: failure.Message}
}

// @HandlesError
// @ResponseStatus(404)
func (handler *MessagingErrorHandler) Missing(failure *NotFound) ApiError {
	return ApiError{Code: "not_found", Message: failure.Message}
}

// @HandlesError
// @ResponseStatus(409)
func (handler *MessagingErrorHandler) Conflict(failure *Conflict) ApiError {
	return ApiError{Code: "conflict", Message: failure.Message}
}

// @HandlesError
// @ResponseStatus(409)
func (handler *MessagingErrorHandler) RecipientsChanged(failure *ChangedRecipients) RecipientsChanged {
	return RecipientsChanged{Code: "recipients_changed", Message: failure.Message, Addresses: append([]string{}, failure.Addresses...)}
}

// @HandlesError
// @ResponseStatus(413)
func (handler *MessagingErrorHandler) TooLarge(failure *TooLarge) ApiError {
	return ApiError{Code: "too_large", Message: failure.Message}
}

// @HandlesError
// @ResponseStatus(422)
func (handler *MessagingErrorHandler) ChatNotAllowed(failure *ChatNotAllowed) ApiError {
	return ApiError{Code: "chat_not_allowed", Message: failure.Message, Field: failure.Address}
}

// @HandlesError
// @ResponseStatus(503)
func (handler *MessagingErrorHandler) Unavailable(failure *Unavailable) ApiError {
	return ApiError{Code: "unavailable", Message: failure.Message}
}
