// UsersHelper reads and writes user documents. Every document it writes is
// normalized first (see UsersRecords).
// @Helper
type UsersHelper struct {
	// @Needs
	fyloHelper *FyloHelper
}

// List answers every user as {docId, ...user}, normalized, in FYLO's order.
func (helper *UsersHelper) List() ([]map[string]any, error) {
	documents, err := helper.store().All()
	if err != nil {
		return nil, err
	}
	users := []map[string]any{}
	for _, document := range documents {
		user := UsersRecords{}.Normalized(document.Data)
		users = append(users, FyloRecord{Id: document.Id, Data: user}.WithDocId())
	}
	return users, nil
}

// FindByEmail answers the document id and user whose primary address or
// alias is email (case-insensitive), or "" and nil.
func (helper *UsersHelper) FindByEmail(email string) (string, map[string]any, error) {
	normalized := Security{}.NormalizeEmailAddress(email)
	if normalized == "" {
		return "", nil, nil
	}
	record, err := helper.store().First(helper.emailField(), normalized)
	if err != nil {
		return "", nil, err
	}
	if record.Id != "" {
		return record.Id, UsersRecords{}.Normalized(record.Data), nil
	}
	return helper.aliased(normalized)
}

func (helper *UsersHelper) aliased(address string) (string, map[string]any, error) {
	users, err := helper.List()
	if err != nil {
		return "", nil, err
	}
	for _, user := range users {
		if aliases, _ := user[helper.aliasesField()].([]string); (PreferenceChoices{}).Includes(aliases, address) {
			return RecordValues{}.Text(user[helper.documentIdField()]), StoredRecords{}.Without(user, helper.documentIdField()), nil
		}
	}
	return "", nil, nil
}

// Put stores a new user and answers its document id.
func (helper *UsersHelper) Put(user map[string]any) (string, error) {
	return helper.store().Put(UsersRecords{}.Normalized(user))
}

// UpsertByEmail creates the user, or merges it into the user with the same
// primary address, and answers the document id. Settings the stored user
// already chose are kept unless the new user names them.
func (helper *UsersHelper) UpsertByEmail(user map[string]any) (string, error) {
	normalized := UsersRecords{}.Normalized(user)
	documentId, existing, err := helper.FindByEmail(RecordValues{}.Text(normalized[helper.emailField()]))
	if err != nil {
		return "", err
	}
	if documentId == "" || existing == nil {
		return helper.Put(normalized)
	}
	merged := UsersMerge{Existing: existing, Normalized: normalized, Given: user}.Merged()
	return documentId, helper.store().Patch(documentId, UsersRecords{}.Normalized(merged))
}

// Delete deletes one user document.
func (helper *UsersHelper) Delete(documentId string) error {
	return helper.store().Delete(documentId)
}

// UpdateAliases replaces the aliases of one user.
func (helper *UsersHelper) UpdateAliases(documentId string, aliases []any, email string) error {
	return helper.store().Patch(documentId, map[string]any{helper.aliasesField(): UsersRecords{}.Aliases(aliases, email)})
}

// UpdatePreferences writes the display-only preferences of user to its
// document, without creating a new one, and answers the normalized user.
func (helper *UsersHelper) UpdatePreferences(documentId string, user map[string]any) (map[string]any, error) {
	normalized := UsersRecords{}.Normalized(user)
	return normalized, helper.store().Patch(documentId, UsersMerge{Normalized: normalized}.Preferences())
}

// UpdateMcpPreferences replaces the MCP preferences of one user.
func (helper *UsersHelper) UpdateMcpPreferences(documentId string, preferences any) error {
	return helper.store().Patch(documentId, map[string]any{"mcp": preferences})
}

// UpdateCategoryCatalog replaces the category catalog of one user with an
// already-normalized catalog.
func (helper *UsersHelper) UpdateCategoryCatalog(documentId string, catalog any) error {
	return helper.store().Patch(documentId, map[string]any{"categoryCatalog": catalog})
}

func (UsersHelper) emailField() string { return "email" }

func (UsersHelper) aliasesField() string { return "aliases" }

func (UsersHelper) documentIdField() string { return "docId" }

func (helper *UsersHelper) store() FyloCollection {
	return FyloCollection{Database: helper.fyloHelper.Database(), Name: FyloCollections{}.Named().Users}
}
