// TextList reads the lists of text stored records hold, such as a user's
// domains or aliases.
type TextList struct{}

// Of answers the text items of value, in order; anything else is skipped.
func (TextList) Of(value any) []string {
	list, _ := RecordValues{}.List(value)
	texts := []string{}
	for _, item := range list {
		if text, isText := item.(string); isText {
			texts = append(texts, text)
		}
	}
	return texts
}

// Distinct answers list without repeated items, keeping the first of each.
func (TextList) Distinct(list []string) []string {
	distinct := []string{}
	for _, item := range list {
		if !(PreferenceChoices{}).Includes(distinct, item) {
			distinct = append(distinct, item)
		}
	}
	return distinct
}
