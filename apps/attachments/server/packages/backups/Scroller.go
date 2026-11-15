package backups

type Scroller func(collection string, where map[string]any, cursor string, size int) (Window, error)
