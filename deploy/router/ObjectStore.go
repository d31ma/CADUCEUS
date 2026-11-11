package main

import "context"

type ObjectStore interface {
	Get(context context.Context, key string) (Reply, error)
}
