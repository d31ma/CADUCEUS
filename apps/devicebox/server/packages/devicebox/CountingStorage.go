package devicebox

import "slices"

type CountingStorage struct {
	Inner   Storage
	writes  *int
	queries *int
	reads   *int
}

func NewCountingStorage(inner Storage) CountingStorage {
	return CountingStorage{Inner: inner, writes: new(int), queries: new(int), reads: new(int)}
}

func (counting CountingStorage) Command(command map[string]any) (any, error) {
	counting.count(counting.isWrite(command["command"]))
	return counting.Inner.Command(command)
}

func (counting CountingStorage) Query(sql string, parameters []any) (any, error) {
	*counting.queries++
	return counting.Inner.Query(sql, parameters)
}

func (counting CountingStorage) Writes() int {
	return *counting.writes
}

func (counting CountingStorage) Queries() int {
	return *counting.queries
}

func (counting CountingStorage) Reads() int {
	return *counting.reads
}

func (counting CountingStorage) Reset() {
	*counting.writes, *counting.queries, *counting.reads = 0, 0, 0
}

func (counting CountingStorage) count(isWrite bool) {
	if isWrite {
		*counting.writes++
		return
	}
	*counting.reads++
}

func (counting CountingStorage) isWrite(name any) bool {
	return slices.Contains(counting.writeCommands(), name)
}

func (CountingStorage) writeCommands() []any {
	return []any{"insert", "patch", "deleteById", "createCollection", "dropCollection"}
}
