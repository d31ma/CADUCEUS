package outbox

type Store interface {
	Pending(limit int) ([]Document, error)
	Load(ref Ref) (Fields, error)
	Save(ref Ref, fields Fields) error
}
