import (
	"fmt"
	"net/http"
	"sync"
)

// RemoteImagesRoute answers POST /inbox/{id}/images: the remote images of
// one message its owner may read, fetched through the server so the browser
// never contacts the image hosts. Only image URLs the stored message holds
// are fetched, so the route is no general web proxy.
type RemoteImagesRoute struct {
	Parts      MailboxParts
	RateLimits *RateLimitService
}

// Fetch answers the fetched images and how many failed.
func (route RemoteImagesRoute) Fetch(id string, request *http.Request) *http.Response {
	caller, refusal := route.Parts.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	if id == "" {
		return Answer{}.NotFound("Email not found")
	}
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return refusal
	}
	requested, valid := route.requested(body["urls"])
	if !valid {
		return Answer{}.BadRequest(fmt.Sprintf("urls must contain between 1 and %d remote image URLs", RemoteImages{}.MostPerRequest()))
	}
	_, email, refusal := route.Parts.Owned(caller, id)
	if refusal != nil {
		return refusal
	}
	limit, err := route.RateLimits.Check([]string{"remote-images", caller.Email}, 60, 60_000)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	if !limit.Allowed {
		return Answer{}.RateLimited("Too many remote image requests", int(limit.RetryAfterSeconds))
	}
	allowed := RemoteImages{}.Urls(email["htmlBody"])
	for _, address := range requested {
		if address == "" || !(TextValue{}).Includes(allowed, address) {
			return Answer{}.BadRequest("Every URL must be an image resource from this message")
		}
	}
	images := route.fetched(requested)
	return Answer{}.Ok(map[string]any{"images": images, "failed": len(requested) - len(images)})
}

// requested answers the distinct normalised URLs of a valid list.
func (RemoteImagesRoute) requested(value any) ([]string, bool) {
	list, isList := value.([]any)
	if !isList || len(list) < 1 || len(list) > (RemoteImages{}).MostPerRequest() {
		return nil, false
	}
	addresses := []string{}
	for _, item := range list {
		text, isText := item.(string)
		if !isText {
			return nil, false
		}
		addresses = append(addresses, (WebAddress{}).Href(text))
	}
	return MailAddresses{}.Unique(addresses), true
}

// fetched fetches every URL at once and answers the images that arrived, in
// request order.
func (RemoteImagesRoute) fetched(addresses []string) []map[string]any {
	results := make([]map[string]any, len(addresses))
	var group sync.WaitGroup
	for index, address := range addresses {
		group.Add(1)
		go func() {
			defer group.Done()
			results[index], _ = RemoteImages{}.Fetched(address)
		}()
	}
	group.Wait()
	images := []map[string]any{}
	for _, image := range results {
		if image != nil {
			images = append(images, image)
		}
	}
	return images
}
