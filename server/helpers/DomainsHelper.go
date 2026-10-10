// DomainsHelper reads and writes hosted domain configurations.
// @Helper
type DomainsHelper struct {
	// @Needs
	fyloHelper *FyloHelper
}

// List answers the configurations of the allowed domains.
func (helper *DomainsHelper) List(allowedDomains []string) ([]map[string]any, error) {
	documents, err := helper.store().All()
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{}
	for _, domain := range allowedDomains {
		allowed[domain] = true
	}
	configs := []map[string]any{}
	for _, document := range documents {
		config := DomainRecords{}.Decoded(document.Data)
		if name, _ := config["domain"].(string); allowed[name] {
			configs = append(configs, config)
		}
	}
	return configs, nil
}

// FindEntry answers the document id and configuration of one domain, or ""
// and nil when it has none.
func (helper *DomainsHelper) FindEntry(domain string) (string, map[string]any, error) {
	documents, err := helper.store().Equal("domain", domain)
	if err != nil || len(documents) == 0 {
		return "", nil, err
	}
	return documents[0].Id, DomainRecords{}.Decoded(documents[0].Data), nil
}

// Put stores a configuration and answers its document id.
func (helper *DomainsHelper) Put(config map[string]any) (string, error) {
	return helper.store().Put(DomainRecords{}.Encoded(config))
}

// UpdateRoutes replaces the inbound routes of one domain document.
func (helper *DomainsHelper) UpdateRoutes(documentId string, routes any) error {
	return helper.store().Patch(documentId, map[string]any{"routes": StoredRecords{}.JsonText(routes)})
}

// Delete deletes one domain document.
func (helper *DomainsHelper) Delete(documentId string) error {
	return helper.store().Delete(documentId)
}

func (helper *DomainsHelper) store() CollectionStore {
	return CollectionStore{Database: helper.fyloHelper.Database(), Collection: FyloCollections{}.Named().Domains}
}
