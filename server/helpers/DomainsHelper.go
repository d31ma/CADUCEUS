// DomainsHelper reads and writes hosted domain configurations.
// @Helper
type DomainsHelper struct {
	// @Needs
	fyloHelper *FyloHelper
}

// List answers the configurations of the allowed domains.
func (helper *DomainsHelper) List(allowedDomains []string) ([]map[string]any, error) {
	documents, err := helper.fyloHelper.Database().FindDocs(FyloCollections{}.Named().Domains, map[string]any{"$ops": []any{}})
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

// Put stores a configuration and answers its document id.
func (helper *DomainsHelper) Put(config map[string]any) (string, error) {
	return helper.fyloHelper.Database().PutData(FyloCollections{}.Named().Domains, DomainRecords{}.Encoded(config))
}
