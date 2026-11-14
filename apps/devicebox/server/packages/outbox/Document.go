package outbox

type Document struct {
	Ref    Ref
	Fields Fields
}
