package storage

type Scroll func(collection string, where map[string]any, cursor string, size int) (Window, error)
