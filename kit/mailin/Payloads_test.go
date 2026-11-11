package mailin

import "testing"

func TestThePayloadUsesAddressStringsAndEmptyLists(t *testing.T) {
	envelope := Envelope{
		Headers:     Headers{From: Mailbox{Name: "Dan", Address: "dan@outside.test"}, To: []Mailbox{{Name: "Ana", Address: "ana@example.com"}}},
		AuthResults: AuthResults{Spf: "none", Dkim: "none", Dmarc: "none", Handoff: true},
	}
	stored := InboundRaw{ID: "t1", Recipients: []string{"ana@example.com"}, RawBlobKey: "outside.test/x"}
	payload := Payloads{}.EmailReceived(stored, envelope)
	check(t, payload.InboundId == "in_t1", "wrong inbound id")
	check(t, payload.Rcpts[0] == "ana@example.com", "wrong recipients")
	check(t, payload.Headers.From == "dan@outside.test", "wrong from")
	check(t, payload.Headers.To[0] == "ana@example.com", "wrong to")
	check(t, payload.Headers.Cc != nil && len(payload.Headers.Cc) == 0, "cc must be an empty list")
	check(t, payload.Headers.References != nil, "references must be a list")
	check(t, payload.RawBlobKey == "outside.test/x", "wrong key")
	check(t, payload.AuthResults.Handoff, "handoff lost")
}
