import (
	"strings"
)

// SmtpFailure is a delivery that did not reach every recipient: the
// per-domain results, the recipients that were and were not delivered, and
// the reply code that classifies the whole failure (550 when every failure
// was permanent, else 451). A failure without results (a transport error)
// carries only Message and ResponseCode.
type SmtpFailure struct {
	Message      string
	Results      []map[string]any
	Delivered    []string
	Failed       []string
	ResponseCode int
}

func (failure *SmtpFailure) Error() string {
	return failure.Message
}

// FromResults answers the failure of a direct delivery's results.
func (SmtpFailure) FromResults(results []map[string]any) *SmtpFailure {
	direct := SmtpDirect{}
	failed, summaries, delivered, failedRecipients := []map[string]any{}, []string{}, []string{}, []string{}
	permanent := true
	for _, result := range results {
		delivered = append(delivered, DeliveryFields{}.Strings(result["deliveredRecipients"])...)
		if len(DeliveryFields{}.Strings(result["failedRecipients"])) == 0 {
			continue
		}
		failed = append(failed, result)
		failedRecipients = append(failedRecipients, DeliveryFields{}.Strings(result["failedRecipients"])...)
		reason := RecordValues{}.Text(result["error"])
		if reason == "" {
			reason = "recipient rejected"
		}
		summaries = append(summaries, RecordValues{}.Text(result["domain"])+": "+reason)
		for _, failure := range direct.recipientFailures(result) {
			permanent = permanent && direct.permanent(failure["responseCode"])
		}
	}
	code := 451
	if permanent {
		code = 550
	}
	return &SmtpFailure{
		Message: "Direct SMTP delivery failed — " + strings.Join(summaries, "; "),
		Results: results, Delivered: direct.uniqueRecipients(delivered), Failed: direct.uniqueRecipients(failedRecipients), ResponseCode: code,
	}
}
