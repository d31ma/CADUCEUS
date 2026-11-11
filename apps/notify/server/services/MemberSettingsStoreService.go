// @Service
type MemberSettingsStoreService struct {
	// @Needs
	documents *DocumentsService
	// @Needs
	names *CollectionNamesService
}

func (store *MemberSettingsStoreService) Get(domainId string, memberId string) (MemberSettingsRecord, error) {
	found := []MemberSettingsRecord{}
	failure := store.documents.Matching(store.names.MemberSettings(domainId), store.byMember(memberId), 1, &found)
	return append(found, store.defaults(domainId, memberId))[0], failure
}

func (store *MemberSettingsStoreService) ForMembers(domainId string, memberIds []string) (map[string]MemberSettingsRecord, error) {
	found := []MemberSettingsRecord{}
	failure := store.documents.Matching(store.names.MemberSettings(domainId), map[string]any{}, store.listLimit(), &found)
	return store.picked(store.byMemberId(found), domainId, memberIds), failure
}

func (store *MemberSettingsStoreService) PutPreferences(domainId string, memberId string, settings NotifySettings) error {
	record := store.defaults(domainId, memberId)
	record.EmailSummaryAfterMinutes, record.MutedConversationIds = settings.EmailSummaryAfterMinutes, settings.MutedConversationIds
	patch := MemberPreferencesPatch{EmailSummaryAfterMinutes: settings.EmailSummaryAfterMinutes, MutedConversationIds: settings.MutedConversationIds}
	return store.documents.Upsert(store.names.MemberSettings(domainId), store.byMember(memberId), record, patch)
}

func (store *MemberSettingsStoreService) PutProfile(domainId string, memberId string, profile MemberProfilePatch) error {
	record := store.defaults(domainId, memberId)
	record.DisplayName, record.PrimaryAddress, record.State = profile.DisplayName, profile.PrimaryAddress, profile.State
	return store.documents.Upsert(store.names.MemberSettings(domainId), store.byMember(memberId), record, profile)
}

func (store *MemberSettingsStoreService) byMember(memberId string) map[string]any {
	return map[string]any{"memberId": memberId}
}

func (store *MemberSettingsStoreService) defaults(domainId string, memberId string) MemberSettingsRecord {
	return MemberSettingsRecord{DomainId: domainId, MemberId: memberId, MutedConversationIds: []string{}}
}

func (store *MemberSettingsStoreService) byMemberId(records []MemberSettingsRecord) map[string]MemberSettingsRecord {
	keyed := map[string]MemberSettingsRecord{}
	for _, record := range records {
		keyed[record.MemberId] = record
	}
	return keyed
}

func (store *MemberSettingsStoreService) picked(keyed map[string]MemberSettingsRecord, domainId string, memberIds []string) map[string]MemberSettingsRecord {
	picked := map[string]MemberSettingsRecord{}
	for _, memberId := range memberIds {
		picked[memberId] = store.orDefault(keyed, domainId, memberId)
	}
	return picked
}

func (store *MemberSettingsStoreService) orDefault(keyed map[string]MemberSettingsRecord, domainId string, memberId string) MemberSettingsRecord {
	record, found := keyed[memberId]
	if found {
		return record
	}
	return store.defaults(domainId, memberId)
}

func (store *MemberSettingsStoreService) listLimit() int {
	return 1000
}
