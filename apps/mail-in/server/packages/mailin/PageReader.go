package mailin

type PageReader func(collection string, where map[string]any, cursor string, limit int) (Page, error)
