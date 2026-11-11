package main

import "context"

type SlotEntry interface {
	Take(caller context.Context, slot chan struct{}) (func(), error)
}
