import (
	"math"
	"net/http"
	"strings"
)

// TestSeedTrackingRoute answers POST /test/seed/tracking: it records one
// recipient open or click on the caller's sent message, for real-browser
// coverage. It answers only when the deployment enables the test routes.
type TestSeedTrackingRoute struct {
	Access   *AccessHelper
	Emails   *EmailsHelper
	Tracking *TrackingHelper
}

// Record records the signal the body names.
func (route TestSeedTrackingRoute) Record(request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return refusal
	}
	if !(Security{}).TestRoutesEnabled() {
		return Answer{}.Forbidden("Only available in test mode")
	}
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	messageId := ""
	if text, isText := body["messageId"].(string); isText {
		messageId = Security{}.Trim(text)
	}
	kind, _ := body["type"].(string)
	if kind != "open" && kind != "click" {
		kind = ""
	}
	if messageId == "" || kind == "" {
		return Answer{}.BadRequest("messageId and type=open|click required")
	}
	_, email, err := route.Emails.FindById(messageId)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	owner, isText := email["ownerEmail"].(string)
	if email == nil || email["folder"] != "sent" || !isText || strings.ToLower(owner) != strings.ToLower(caller.Email) {
		return Answer{}.NotFound("Sent email not found")
	}
	recorded, err := route.recorded(kind, email, body)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	if !recorded {
		return Answer{}.BadRequest("Message does not have " + kind + " tracking enabled")
	}
	return Answer{}.Ok(map[string]any{"recorded": true, "type": kind, "messageId": messageId})
}

func (route TestSeedTrackingRoute) recorded(kind string, email map[string]any, body map[string]any) (bool, error) {
	trackingId := RecordValues{}.Text(RecordValues{}.Or(email["trackingId"], ""))
	recipient := route.recipient(email, body)
	if kind == "open" {
		return route.Tracking.RecordOpen(trackingId, recipient)
	}
	index := RecordValues{}.Numeric(body["linkIndex"])
	if math.IsNaN(index) || math.IsInf(index, 0) || index != math.Trunc(index) {
		index = 0
	}
	links, _ := RecordValues{}.List(email["trackedLinks"])
	for _, item := range links {
		link, _ := RecordValues{}.Object(item)
		if position, isNumber := (RecordValues{}).Number(link["index"]); isNumber && position == index {
			return route.click(trackingId, recipient, link, index)
		}
	}
	return false, nil
}

func (route TestSeedTrackingRoute) click(trackingId string, recipient string, link map[string]any, index float64) (bool, error) {
	if !(RecordValues{}).Truthy(link["url"]) {
		return false, nil
	}
	url, _ := link["url"].(string)
	return route.Tracking.RecordClick(TrackingClick{TrackingId: trackingId, Recipient: recipient, Url: url, LinkIndex: index})
}

// recipient answers the recipient the body names, else the message's first.
func (TestSeedTrackingRoute) recipient(email map[string]any, body map[string]any) string {
	if named, isText := body["recipient"].(string); isText {
		return named
	}
	if recipients, _ := (RecordValues{}).List(email["to"]); len(recipients) > 0 && (RecordValues{}).Truthy(recipients[0]) {
		return BodyText{}.String(recipients[0])
	}
	return strings.Split(BodyText{}.String(RecordValues{}.Or(email["recipient"], "")), ",")[0]
}
