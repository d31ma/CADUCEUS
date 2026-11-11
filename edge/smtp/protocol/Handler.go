package protocol

import "context"

type Handler func(ctx context.Context, command Command) Reply
