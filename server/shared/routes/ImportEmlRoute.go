import (
	"net/http"
	"strings"
)

// ImportEmlRoute answers POST /inbox/import/eml: one EML file imported into
// one mailbox the boundary lets the caller write.
type ImportEmlRoute struct {
	Access     *AccessHelper
	Categories *CategoryMappingService
	Imports    *EmlImportService
}

// Import validates and stores the file.
func (route ImportEmlRoute) Import(request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return refusal
	}
	named, _ := body["mailbox"].(string)
	caller, refusal := route.Access.Allowed(request, RouteAccess{}.MailboxWrite(), "", named)
	if refusal != nil {
		return refusal
	}
	mailbox := Security{}.NormalizeEmailAddress(named)
	if mailbox == "" {
		return Answer{}.BadRequest("mailbox is required")
	}
	domain := strings.SplitN(mailbox, "@", 2)[1]
	if domain != caller.Domain {
		return Answer{}.Forbidden("Access denied")
	}
	upload := EmlUpload{}
	folder, err := upload.Folder(body["folder"])
	if err != nil {
		return Answer{}.BadRequest(err.Error())
	}
	source, err := upload.Source(body["contentBase64"])
	if err != nil {
		return Answer{}.BadRequest(err.Error())
	}
	imported, err := route.imported(caller, source, domain, mailbox, folder)
	if err != nil {
		return Answer{}.Unavailable("The EML file could not be imported")
	}
	imported["imported"] = true
	return Answer{}.Ok(imported)
}

func (route ImportEmlRoute) imported(caller RouteCaller, source []byte, domain string, mailbox string, folder string) (map[string]any, error) {
	mappings, err := route.Categories.ForOwner(caller.Email)
	if err != nil {
		return nil, err
	}
	if path := mappings.PathFor(domain, folder); path != "" {
		folder = path
	}
	return route.Imports.ImportOne(source, domain, mailbox, folder)
}
