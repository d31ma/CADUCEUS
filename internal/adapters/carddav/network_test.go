package carddav

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/d31ma/caduceus/internal/accounts"
)

func TestSafeClientRejectsMetadataAndUnapprovedPrivateDestinations(t *testing.T) {
	for _, address := range []string{"169.254.169.254", "127.0.0.1", "10.20.30.40"} {
		client, err := NewClient(Options{Resolver: fixedResolver{addresses: []net.IPAddr{{IP: net.ParseIP(address)}}}})
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.Discover(context.Background(), &accounts.CardDAVEndpoint{URL: "https://dav.example/", Username: "alice"}, accounts.Credentials{Password: "secret"})
		if !errors.Is(err, ErrUnsafeEndpoint) {
			t.Fatalf("address %s error=%v, want ErrUnsafeEndpoint", address, err)
		}
	}
}

func TestSafeClientRequiresCanonicalBoundedPrivateNetworks(t *testing.T) {
	for _, cidr := range []string{"10.20.1.1/16", "0.0.0.0/0", "127.0.0.0/4", "not-a-cidr"} {
		if _, err := NewClient(Options{PrivateCIDRs: []string{cidr}}); err == nil {
			t.Fatalf("NewClient(%q) succeeded", cidr)
		}
	}
	if _, err := NewClient(Options{PrivateCIDRs: []string{"127.0.0.0/8", "10.20.0.0/16", "::1/128"}}); err != nil {
		t.Fatalf("approved private policy: %v", err)
	}
}

type fixedResolver struct{ addresses []net.IPAddr }

func (resolver fixedResolver) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	return resolver.addresses, nil
}
