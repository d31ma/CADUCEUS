import (
	"errors"
	"net/http"
	"regexp"
)

// ThreadSharesRoute answers /thread-shares and its public links: the
// owner's expiring conversation links and what a link shows.
type ThreadSharesRoute struct {
	Access  *AccessHelper
	Sharing *ThreadSharingService
}

// List answers the caller's links, optionally for one authorized thread.
func (route ThreadSharesRoute) List(request *http.Request) *http.Response {
	caller, refusal := route.Access.Allowed(request, RouteAccess{}.MailboxRead(), "", "")
	if refusal != nil {
		return refusal
	}
	emailId := ""
	if values := request.URL.Query()["emailId"]; len(values) > 0 {
		emailId = Security{}.Trim(values[len(values)-1])
	}
	if (TextValue{}).Length(emailId) > 500 {
		return Answer{}.BadRequest("emailId is too long")
	}
	shares, err := route.Sharing.List(caller.Email, emailId, caller.Domains)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	if shares == nil {
		return Answer{}.NotFound("Conversation not found")
	}
	return Answer{}.Ok(shares)
}

// Create mints one expiring link and revokes the thread's earlier one.
func (route ThreadSharesRoute) Create(request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Allowed(request, RouteAccess{}.MailboxRead(), "", "")
	if refusal != nil {
		return refusal
	}
	created, err := route.Sharing.Create(ThreadShareRequest{OwnerEmail: caller.Email, Domains: caller.Domains, EmailId: body["emailId"], ExpiresInDays: body["expiresInDays"]})
	var input *ThreadShareInputError
	if errors.As(err, &input) {
		return Answer{}.BadRequest(input.Message)
	}
	return route.found(created, err, "Conversation not found")
}

// Revoke revokes one of the caller's links.
func (route ThreadSharesRoute) Revoke(id string, request *http.Request) *http.Response {
	if _, refusal := (RequestBody{}).Value(request); refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Allowed(request, RouteAccess{}.MailboxRead(), "", "")
	if refusal != nil {
		return refusal
	}
	if !regexp.MustCompile(`^share-[A-Za-z0-9_-]{20,40}$`).MatchString(id) {
		return Answer{}.NotFound("Share not found")
	}
	revoked, err := route.Sharing.Revoke(id, caller.Email)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	if !revoked {
		return Answer{}.NotFound("Share not found")
	}
	return Answer{}.Ok(map[string]any{"revoked": id})
}

// Show answers the public view of a link; invalid, expired and revoked
// links answer alike.
func (route ThreadSharesRoute) Show(token string) *http.Response {
	view, err := route.Sharing.Resolve(token)
	return route.found(view, err, "Shared conversation not found")
}

// Download answers one attachment of a shared conversation.
func (route ThreadSharesRoute) Download(token string, attachmentId string) *http.Response {
	download, err := route.Sharing.ReadAttachment(token, attachmentId)
	return route.found(download, err, "Shared attachment not found")
}

func (ThreadSharesRoute) found(value map[string]any, err error, missing string) *http.Response {
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	if value == nil {
		return Answer{}.NotFound(missing)
	}
	return Answer{}.Ok(value)
}
