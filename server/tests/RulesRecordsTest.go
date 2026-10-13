// @Test(MailboxTestSetup)
type RulesRecordsTest struct {
	behaviors *BehaviorContext
}

func (test *RulesRecordsTest) rule(id string, name string, order any) map[string]any {
	return map[string]any{"id": id, "name": name, "order": order}
}

func (test *RulesRecordsTest) TestStoresListsAsJsonText() {
	stored := RulesRecords{}.Serialize(map[string]any{"id": "r1", "conditions": []any{map[string]any{"value": "a<b"}}})
	Assert.Equals(`[{"value":"a<b"}]`, stored["conditions"])
	Assert.Equals("[]", stored["actions"])
}

func (test *RulesRecordsTest) TestReadsAStoredRule() {
	rule, err := RulesRecords{}.Deserialize(map[string]any{"ownerEmail": "Alice@Example.com", "order": "2", "conditions": "[1]"})
	Assert.Equals(nil, err)
	Assert.Equals("alice@example.com", rule["ownerEmail"])
	Assert.Equals(2.0, rule["order"])
	Assert.Equals(false, rule["stopProcessing"])
	Assert.Equals([]any{1.0}, rule["conditions"])
	Assert.Equals([]any{}, rule["actions"])
}

func (test *RulesRecordsTest) TestRefusesMalformedConditions() {
	_, err := RulesRecords{}.Deserialize(map[string]any{"conditions": "[oops"})
	Assert.IsTrue(err != nil)
}

func (test *RulesRecordsTest) TestPatchesOnlyChangeableFields() {
	patch := RulesRecords{}.SerializePatch(map[string]any{"name": "n", "domain": "x.com", "actions": []any{}})
	Assert.Equals(map[string]any{"name": "n", "actions": "[]"}, patch)
}

func (test *RulesRecordsTest) TestOrdersByOrderThenNameThenId() {
	rules := []map[string]any{test.rule("b", "beta", 1.0), test.rule("a", "Beta", 1.0), test.rule("c", "Alpha", 1.0), test.rule("d", "zed", 0.0)}
	RulesRecords{}.Sort(rules)
	Assert.Equals([]any{"d", "c", "b", "a"}, []any{rules[0]["id"], rules[1]["id"], rules[2]["id"], rules[3]["id"]})
}

func (test *RulesRecordsTest) TestKeepsOwnerlessRulesDomainWide() {
	rules := []map[string]any{
		{"id": "1", "name": "a", "ownerEmail": "alice@example.com", "mailbox": "sales@example.com"},
		{"id": "2", "name": "b"},
		{"id": "3", "name": "c", "ownerEmail": "bob@example.com"},
		{"id": "4", "name": "d", "ownerEmail": "alice@example.com"},
	}
	kept := RulesRecords{}.ForMailbox(rules, " Alice@example.com", "alice@example.com")
	Assert.Equals(2, len(kept))
	Assert.Equals("2", kept[0]["id"])
	Assert.Equals("4", kept[1]["id"])
}

func (test *RulesRecordsTest) TestCountsTheNextOrder() {
	Assert.Equals(0.0, RulesRecords{}.NextOrder(nil))
	Assert.Equals(4.0, RulesRecords{}.NextOrder([]map[string]any{test.rule("a", "a", 3.0), test.rule("b", "b", "x")}))
}

func (test *RulesRecordsTest) TestAcceptsOnlyTheOwnersExactRules() {
	rules := []map[string]any{test.rule("a", "a", 0.0), test.rule("b", "b", 1.0)}
	records := RulesRecords{}
	Assert.IsTrue(records.SameRules(rules, []string{"b", "a"}))
	Assert.IsTrue(!records.SameRules(rules, []string{"a", "a"}))
	Assert.IsTrue(!records.SameRules(rules, []string{"a"}))
	Assert.IsTrue(!records.SameRules(rules, []string{"a", "c"}))
}

func (test *RulesRecordsTest) TestComparesLikeTheRootLocale() {
	order := LocaleOrder{}
	Assert.IsTrue(order.Compare("apple", "Banana") < 0)
	Assert.IsTrue(order.Compare("a", "A") < 0)
	Assert.IsTrue(order.Compare("_x", "1x") < 0)
	Assert.Equals(0, order.Compare("same", "same"))
}
