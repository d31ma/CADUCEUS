package realtime

import (
	"strings"
	"testing"
)

func TestAllowedAcceptsTheConfiguredSecret(t *testing.T) {
	secret := strings.Repeat("s", 32)
	if !(GatewayCheck{Secret: secret}).Allowed(secret) {
		t.Fatal("the right secret was refused")
	}
}

func TestAllowedRefusesAWrongSecret(t *testing.T) {
	if (GatewayCheck{Secret: strings.Repeat("s", 32)}).Allowed(strings.Repeat("t", 32)) {
		t.Fatal("a wrong secret was allowed")
	}
}

func TestAllowedRefusesASecretShorterThan32Characters(t *testing.T) {
	secret := strings.Repeat("s", 31)
	if (GatewayCheck{Secret: secret}).Allowed(secret) {
		t.Fatal("a short secret was allowed")
	}
}

func TestAllowedRefusesWhenNoSecretIsConfigured(t *testing.T) {
	if (GatewayCheck{}).Allowed("") {
		t.Fatal("an empty secret was allowed")
	}
}
