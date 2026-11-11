package mailout

type Draft struct {
	Item     Item
	Selector string
	Source   MessageSource
	Refusal  string
}
