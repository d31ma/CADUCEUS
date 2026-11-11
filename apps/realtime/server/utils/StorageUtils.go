import "mesail.dev/packages/realtime"

type StorageUtils struct{}

func (StorageUtils) From(dataSource DataSource) realtime.Storage {
	return realtime.Storage{Documents: dataSource, Scroll: StorageUtils{}.scroller(dataSource)}
}

func (StorageUtils) scroller(dataSource DataSource) realtime.Scroller {
	return func(collection string, where map[string]any, cursor string, size int) (realtime.Window, error) {
		window, failure := dataSource.Scroll(collection, where, cursor, size)
		return realtime.Window{Content: window.Content, NextCursor: window.NextCursor}, failure
	}
}
