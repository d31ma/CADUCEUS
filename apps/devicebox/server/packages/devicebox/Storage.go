package devicebox

type Storage interface {
	Command(command map[string]any) (any, error)
	Query(sql string, parameters []any) (any, error)
}
