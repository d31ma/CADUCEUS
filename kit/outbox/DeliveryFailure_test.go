package outbox

import "testing"

func TestDeliveryFailureDescribesItself(t *testing.T) {
	failure := DeliveryFailure{Subscriber: "messaging", Status: 503, Detail: "503 Service Unavailable"}

	if failure.Error() != "delivery to messaging failed (status 503, permanent false): 503 Service Unavailable" {
		t.Fatalf("message %q", failure.Error())
	}
}
