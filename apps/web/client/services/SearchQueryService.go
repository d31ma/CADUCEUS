// @Service
type SearchQueryService struct {
	// @Needs
	client *SearchClient
	// @Needs
	session *SessionService
	// @Needs
	rows *SearchRowService
}

func (service *SearchQueryService) Find(filter SearchHitFilter) SearchHitAnswer {
	token, failure := service.session.AccessToken()
	page, failure := service.pageUnlessFailed(service.queryOf(token, filter), failure)
	page, failure = service.retryAfterRenew(filter, page, failure)
	return service.answerOf(filter, page, failure)
}

func (service *SearchQueryService) answerOf(filter SearchHitFilter, page SearchHitPage, failure error) SearchHitAnswer {
	return SearchHitAnswer{
		Rows:       service.rows.RowsOf(page.Items, filter.Query),
		NextCursor: service.cursorAfter(page, filter, failure),
		State:      service.stateOf(failure, len(page.Items)),
	}
}

func (service *SearchQueryService) queryOf(token string, filter SearchHitFilter) SearchHitQuery {
	return SearchHitQuery{
		Authorization:  service.bearerPrefix() + token,
		Query:          filter.Query,
		ConversationId: filter.ConversationId,
		Author:         filter.Author,
		From:           filter.From,
		To:             filter.To,
		HasAttachment:  service.flagText(filter.HasAttachment),
		IsEmail:        service.flagText(filter.IsEmail),
		Cursor:         filter.Cursor,
		Limit:          filter.Limit,
	}
}

func (service *SearchQueryService) pageUnlessFailed(query SearchHitQuery, failure error) (SearchHitPage, error) {
	if failure != nil {
		return SearchHitPage{}, failure
	}
	return service.ask(query)
}

func (service *SearchQueryService) retryAfterRenew(filter SearchHitFilter, page SearchHitPage, failure error) (SearchHitPage, error) {
	if BehaviorErrorStatus(failure) != service.unauthorizedStatus() {
		return page, failure
	}
	return service.askAgain(filter)
}

func (service *SearchQueryService) askAgain(filter SearchHitFilter) (SearchHitPage, error) {
	token, failure := service.session.Renew()
	return service.pageUnlessFailed(service.queryOf(token, filter), service.endedWhenFailed(failure))
}

func (service *SearchQueryService) endedWhenFailed(failure error) error {
	if failure == nil {
		return nil
	}
	return &BehaviorError{Code: service.sessionEndedCode(), Message: service.sessionEndedMessage(), Status: service.unauthorizedStatus()}
}

func (service *SearchQueryService) ask(query SearchHitQuery) (SearchHitPage, error) {
	return BehaviorsAnswer[SearchHitPage](service.client.Search(
		query.Authorization,
		query.Query,
		query.ConversationId,
		query.Author,
		query.From,
		query.To,
		query.HasAttachment,
		query.IsEmail,
		query.Cursor,
		query.Limit,
	))
}

func (service *SearchQueryService) cursorAfter(page SearchHitPage, filter SearchHitFilter, failure error) string {
	if failure != nil {
		return filter.Cursor
	}
	return page.NextCursor
}

func (service *SearchQueryService) stateOf(failure error, count int) string {
	switch {
	case BehaviorErrorStatus(failure) == service.unauthorizedStatus():
		return service.stateSession()
	case failure != nil:
		return service.stateError()
	case count == 0:
		return service.stateEmpty()
	}
	return service.stateResults()
}

func (service *SearchQueryService) flagText(value bool) string {
	if value {
		return "true"
	}
	return ""
}

func (service *SearchQueryService) bearerPrefix() string {
	return "Bearer "
}

func (service *SearchQueryService) unauthorizedStatus() int {
	return 401
}

func (service *SearchQueryService) sessionEndedCode() string {
	return "session_ended"
}

func (service *SearchQueryService) sessionEndedMessage() string {
	return "the session ended and could not be renewed"
}

func (service *SearchQueryService) stateSession() string {
	return "session"
}

func (service *SearchQueryService) stateError() string {
	return "error"
}

func (service *SearchQueryService) stateEmpty() string {
	return "empty"
}

func (service *SearchQueryService) stateResults() string {
	return "results"
}
