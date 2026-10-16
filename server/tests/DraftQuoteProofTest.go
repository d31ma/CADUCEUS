import (
	"encoding/json"
	"strings"
)

// The proof is an explicit draft transport field, never an HTML marker.
// @Test(MailboxTestSetup)
type DraftQuoteProofTest struct {
	behaviors *BehaviorContext
}

// @ValueSource(["reply", "forward"])
func (test *DraftQuoteProofTest) TestAcceptsOnlyTheDeclaredCompactProof(mode string) {
	value := map[string]any{"mode": mode, "sha256": strings.Repeat("a", 64)}
	proof, problem := DraftQuoteProof{}.Parsed(value)
	Assert.Equals("", problem)
	Assert.Equals(value, proof)
	value["mode"] = "changed"
	Assert.Equals(mode, proof["mode"])
}

func (test *DraftQuoteProofTest) TestMissingProofClearsTheCompleteSnapshot() {
	proof, problem := DraftQuoteProof{}.Parsed(nil)
	Assert.Equals("", problem)
	Assert.IsTrue(proof == nil)
	input, problem := DraftInput{}.Parsed(map[string]any{"clientId": "proof-test"})
	Assert.Equals("", problem)
	_, present := input["quoteProof"]
	Assert.IsTrue(present)
	kept, _ := input["quoteProof"].(map[string]any)
	Assert.IsTrue(kept == nil)
}

// @ValueSource(["\"raw marker\"","[]","{}","{\"mode\":\"reply\",\"sha256\":\"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\",\"body\":\"untrusted\"}","{\"mode\":\"other\",\"sha256\":\"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\"}","{\"mode\":true,\"sha256\":\"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\"}","{\"mode\":\"reply\",\"sha256\":7}","{\"mode\":\"reply\",\"sha256\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\"}","{\"mode\":\"reply\",\"sha256\":\"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\"}","{\"mode\":\"reply\",\"sha256\":\"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\"}"])
func (test *DraftQuoteProofTest) TestRejectsUndeclaredMalformedAndOversizedProofs(encoded string) {
	var value any
	failure := json.Unmarshal([]byte(encoded), &value)
	Assert.Equals(nil, failure)
	proof, problem := DraftQuoteProof{}.Parsed(value)
	Assert.IsTrue(proof == nil)
	Assert.IsTrue(problem != "")
}

func (test *DraftQuoteProofTest) TestCompleteDraftCarriesTheValidatedProof() {
	proof := map[string]any{"mode": "reply", "sha256": strings.Repeat("a", 64)}
	input, problem := DraftInput{}.Parsed(map[string]any{"clientId": "proof-test", "quoteProof": proof})
	Assert.Equals("", problem)
	Assert.Equals(proof, input["quoteProof"])
}
