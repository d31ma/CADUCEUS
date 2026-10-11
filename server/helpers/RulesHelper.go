import "strings"

// RulesHelper reads and writes inbox rules.
// @Helper
type RulesHelper struct {
	// @Needs
	fyloHelper *FyloHelper
}

// List answers the rules of the allowed domains in rule order.
func (helper *RulesHelper) List(allowedDomains []string) ([]map[string]any, error) {
	stored, err := helper.rules().All()
	if err != nil {
		return nil, err
	}
	return RulesRecords{}.Sorted(stored, func(raw map[string]any) bool { return TextValue{}.Includes(allowedDomains, raw[helper.domainField()]) })
}

// ListForOwner answers the rules of one exact owner in rule order.
func (helper *RulesHelper) ListForOwner(ownerEmail string) ([]map[string]any, error) {
	stored, err := helper.rules().Where("ownerEmail", strings.ToLower(TextValue{}.Trim(ownerEmail)))
	return helper.sorted(stored, err, func(map[string]any) bool { return true })
}

// listEnabledForDomain answers the enabled rules of one domain in rule order.
func (helper *RulesHelper) listEnabledForDomain(domain string) ([]map[string]any, error) {
	stored, err := helper.rules().Where(helper.domainField(), domain)
	return helper.sorted(stored, err, func(raw map[string]any) bool { return TextValue{}.Truthy(raw["enabled"]) })
}

// ListEnabledForMailbox answers the enabled rules that apply to one mailbox
// of a domain. Historical rules without an owner stay domain-wide.
func (helper *RulesHelper) ListEnabledForMailbox(ownerEmail string, mailbox string, domain string) ([]map[string]any, error) {
	rules, err := helper.listEnabledForDomain(domain)
	if err != nil {
		return nil, err
	}
	return RulesRecords{}.ForMailbox(rules, ownerEmail, mailbox), nil
}

// FindById answers the document id and rule with a rule id, or "" and nil.
func (helper *RulesHelper) FindById(ruleId string) (string, map[string]any, error) {
	record, err := helper.rules().First("id", ruleId)
	if err != nil || record.Id == "" {
		return "", nil, err
	}
	rule, err := RulesRecords{}.Deserialize(record.Data)
	if err != nil {
		return "", nil, err
	}
	return record.Id, rule, nil
}

// Put stores a rule and answers its document id.
func (helper *RulesHelper) Put(rule map[string]any) (string, error) {
	return helper.rules().Put(RulesRecords{}.Serialize(rule))
}

// Update merges the changeable fields of a patch into one stored rule.
func (helper *RulesHelper) Update(documentId string, patch map[string]any) error {
	return helper.rules().Patch(documentId, RulesRecords{}.SerializePatch(patch))
}

// Delete deletes one stored rule.
func (helper *RulesHelper) Delete(documentId string) error {
	return helper.rules().Delete(documentId)
}

// NextOrder answers the order a new rule of the owner takes.
func (helper *RulesHelper) NextOrder(ownerEmail string) (float64, error) {
	rules, err := helper.ListForOwner(ownerEmail)
	return RulesRecords{}.NextOrder(rules), err
}

// Reorder stores the owner's complete rule order, or answers false when ids
// are not exactly the owner's rules.
func (helper *RulesHelper) Reorder(ownerEmail string, ids []string) (bool, error) {
	rules, err := helper.ListForOwner(ownerEmail)
	if err != nil || !(RulesRecords{}).SameRules(rules, ids) {
		return false, err
	}
	patches := []FyloRecord{}
	for order, id := range ids {
		documentId, _, err := helper.FindById(id)
		if err != nil {
			return false, err
		}
		if documentId != "" {
			patches = append(patches, FyloRecord{Id: documentId, Data: map[string]any{"order": order}})
		}
	}
	return true, helper.rules().PatchEach(patches)
}

func (*RulesHelper) sorted(stored []FyloRecord, failure error, keep func(raw map[string]any) bool) ([]map[string]any, error) {
	if failure != nil {
		return nil, failure
	}
	return RulesRecords{}.Sorted(stored, keep)
}

func (helper *RulesHelper) rules() FyloCollection {
	return FyloCollection{Database: helper.fyloHelper.Database(), Name: FyloCollections{}.Named().InboxRules}
}

func (*RulesHelper) domainField() string { return "domain" }
