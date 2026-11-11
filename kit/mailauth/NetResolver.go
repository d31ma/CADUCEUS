package mailauth

import (
	"context"
	"net"
	"time"
)

type NetResolver struct {
	resolver *net.Resolver
}

func NewNetResolver() NetResolver {
	return NetResolver{resolver: net.DefaultResolver}
}

func (netResolver NetResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	bounded, cancel := context.WithTimeout(ctx, netResolver.timeout())
	defer cancel()
	return netResolver.resolver.LookupTXT(bounded, name)
}

func (netResolver NetResolver) LookupIP(ctx context.Context, host string) ([]net.IP, error) {
	bounded, cancel := context.WithTimeout(ctx, netResolver.timeout())
	defer cancel()
	return netResolver.resolver.LookupIP(bounded, netResolver.network(), host)
}

func (netResolver NetResolver) LookupMX(ctx context.Context, name string) ([]*net.MX, error) {
	bounded, cancel := context.WithTimeout(ctx, netResolver.timeout())
	defer cancel()
	return netResolver.resolver.LookupMX(bounded, name)
}

func (NetResolver) timeout() time.Duration {
	return 5 * time.Second
}

func (NetResolver) network() string {
	return "ip"
}
