import (
	"net/http"
	"strings"
)

// SuppressedRoute answers /suppressed: the addresses outbound mail must not
// reach, managed by domain administrators.
type SuppressedRoute struct {
	Access     *AccessHelper
	Suppressed *SuppressedHelper
}

// List answers GET /suppressed.
func (route SuppressedRoute) List(request *http.Request) *http.Response {
	if _, refusal := route.Access.Allowed(request, (RouteAccess{}).DomainAdmin(), "", ""); refusal != nil {
		return refusal
	}
	return DeliveryReply{}.Settled(route.Suppressed.List())
}

// Create answers POST /suppressed: suppress one address manually.
func (route SuppressedRoute) Create(request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return refusal
	}
	if _, refusal := route.Access.Allowed(request, (RouteAccess{}).DomainAdmin(), "", ""); refusal != nil {
		return refusal
	}
	address := ""
	if (RecordValues{}).Truthy(body["address"]) {
		address = strings.ToLower(Security{}.Trim(RecordValues{}.String(body["address"])))
	}
	if address == "" {
		return Answer{}.BadRequest("address required")
	}
	reason := "bounce"
	if body["reason"] == "complaint" {
		reason = "complaint"
	}
	if err := route.Suppressed.Suppress(address, reason); err != nil {
		return DeliveryReply{}.Settled(nil, err)
	}
	return Answer{}.Ok(map[string]any{"suppressed": address, "reason": reason})
}

// Remove answers DELETE /suppressed/{address}.
func (route SuppressedRoute) Remove(address string, request *http.Request) *http.Response {
	if _, refusal := route.Access.Allowed(request, (RouteAccess{}).DomainAdmin(), "", ""); refusal != nil {
		return refusal
	}
	if address == "" {
		return Answer{}.BadRequest("address required")
	}
	if err := route.Suppressed.Delete(address); err != nil {
		return DeliveryReply{}.Settled(nil, err)
	}
	return Answer{}.Ok(map[string]any{"removed": address})
}
