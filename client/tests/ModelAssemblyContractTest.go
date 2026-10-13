import "encoding/json"

// @Test(ShellTestSetup, {"partition":"tests"})
type ModelAssemblyContractTest struct {
	behaviors *BehaviorContext
}

func (test *ModelAssemblyContractTest) TestDraftDiscardAnswerPreservesItsIdentifier() {
	answer := DraftDiscarded{}
	Assert.Equals(nil, json.Unmarshal([]byte(`{"discarded":"draft-1"}`), &answer))
	Assert.Equals("draft-1", answer.Discarded)
}

func (test *ModelAssemblyContractTest) TestSignatureApplyEventPreservesTheBridgePayload() {
	event := SignatureApplyRequested{SignatureID: "signature-1", Text: "Regards"}
	encoded, failure := json.Marshal(event)
	Assert.Equals(nil, failure)
	fields := map[string]any{}
	Assert.Equals(nil, json.Unmarshal(encoded, &fields))
	Assert.Equals(map[string]any{"signatureId": "signature-1", "text": "Regards"}, fields)
}

func (test *ModelAssemblyContractTest) TestSignOutEventPreservesTheSharedPayload() {
	event := SignOutRequested{Reason: "session-ended"}
	encoded, failure := json.Marshal(event)
	Assert.Equals(nil, failure)
	fields := map[string]any{}
	Assert.Equals(nil, json.Unmarshal(encoded, &fields))
	Assert.Equals(map[string]any{"reason": "session-ended"}, fields)
}
