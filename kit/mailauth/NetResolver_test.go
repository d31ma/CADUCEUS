package mailauth

import (
	"context"
	"testing"
)

func TestNetResolverStopsWhenTheContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	resolver := NewNetResolver()
	_, textError := resolver.LookupTXT(ctx, "example.invalid")
	_, addressError := resolver.LookupIP(ctx, "example.invalid")
	_, exchangeError := resolver.LookupMX(ctx, "example.invalid")
	if textError == nil || addressError == nil || exchangeError == nil {
		t.Fatalf("got %v, %v, %v", textError, addressError, exchangeError)
	}
}
