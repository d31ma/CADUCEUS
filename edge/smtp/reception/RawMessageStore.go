package reception

import "context"

type RawMessageStore interface {
	Put(ctx context.Context, key string, content []byte) error
}
