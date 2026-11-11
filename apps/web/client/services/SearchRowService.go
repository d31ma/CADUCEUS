// @Service
type SearchRowService struct {
	// @Needs
	highlights *SearchHighlightService
	// @Needs
	times *SearchTimeService
}

func (service *SearchRowService) RowsOf(hits []SearchHit, query string) []SearchHitRow {
	return service.rowsOf(hits, query)
}

func (service *SearchRowService) rowsOf(hits []SearchHit, query string) []SearchHitRow {
	rows := []SearchHitRow{}
	for _, hit := range hits {
		rows = append(rows, service.rowOf(hit, query))
	}
	return rows
}

func (service *SearchRowService) rowOf(hit SearchHit, query string) SearchHitRow {
	return SearchHitRow{
		MessageId:      hit.MessageId,
		ConversationId: hit.ConversationId,
		CreatedAt:      hit.CreatedAt,
		TimeLabel:      service.times.LabelOf(hit.CreatedAt),
		Parts:          service.highlights.PartsOf(hit.Snippet, query),
	}
}
