package outbox

type Store interface {
	Waiting() ([]Document, error)
	Load(ref Ref) (Fields, error)
	Save(ref Ref, fields Fields) error
}
