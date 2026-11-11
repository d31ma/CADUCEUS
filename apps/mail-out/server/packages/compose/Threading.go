package compose

type Threading struct{}

func (Threading) ReplyReferences(parentReferences []string, parentMessageId string) []string {
	list := referenceList{}
	all := append(append([]string{}, parentReferences...), parentMessageId)
	return list.capped(list.unique(all))
}
