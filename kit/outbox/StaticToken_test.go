package outbox

import "testing"

func TestStaticTokenReturnsItsValue(t *testing.T) {
	token, err := StaticToken{Value: "service-token"}.Token()

	if err != nil || token != "service-token" {
		t.Fatalf("token %q, error %v", token, err)
	}
}
