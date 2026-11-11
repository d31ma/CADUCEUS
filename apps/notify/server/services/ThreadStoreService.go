import "slices"

// @Service
type ThreadStoreService struct {
	// @Needs
	documents *DocumentsService
	// @Needs
	names *CollectionNamesService
}

func (store *ThreadStoreService) AddParticipant(domainId string, threadRootId string, memberId string) error {
	participants, failure := store.Participants(domainId, threadRootId)
	record := ThreadRecord{DomainId: domainId, ThreadRootId: threadRootId, MemberIds: []string{memberId}}
	return store.saveUnless(failure, slices.Contains(participants, memberId), record, append(participants, memberId))
}

func (store *ThreadStoreService) Participants(domainId string, threadRootId string) ([]string, error) {
	found := []ThreadRecord{}
	failure := store.documents.Matching(store.names.Threads(domainId), store.byRoot(threadRootId), 1, &found)
	return append(found, ThreadRecord{MemberIds: []string{}})[0].MemberIds, failure
}

func (store *ThreadStoreService) saveUnless(failure error, known bool, record ThreadRecord, memberIds []string) error {
	if failure != nil || known {
		return failure
	}
	return store.documents.Upsert(store.names.Threads(record.DomainId), store.byRoot(record.ThreadRootId), record, ThreadMembersPatch{MemberIds: memberIds})
}

func (store *ThreadStoreService) byRoot(threadRootId string) map[string]any {
	return map[string]any{"threadRootId": threadRootId}
}
