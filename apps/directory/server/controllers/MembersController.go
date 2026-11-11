import (
	"net/http"
	"strconv"
)

// @RequestRoute("/api/v1/directory/members")
// @SignedIn
type MembersController struct {
	// @Needs
	admission *AdmissionService
	// @Needs
	members *MemberService
}

// @GetRoute
func (controller *MembersController) GetMembers(request *http.Request) (MemberPage, error) {
	admission := controller.admission.Admit(request.Header.Get(controller.authorizationHeader()))
	values := request.URL.Query()
	limit, invalid := controller.limit(values.Get(controller.limitName()))
	query := MemberQuery{Text: values.Get(controller.queryName()), Cursor: values.Get(controller.cursorName()), Limit: limit}
	return controller.members.Search(admission, query, invalid)
}

// @GetRoute("/{member_id}")
func (controller *MembersController) GetMember(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("member_id")
	memberId string,
) (MemberSummary, error) {
	return controller.members.Summary(controller.admission.Admit(authorization), memberId)
}

func (controller *MembersController) limit(raw string) (int, error) {
	limit, failure := strconv.Atoi(raw)
	switch {
	case raw == "":
		return 0, nil
	case failure != nil:
		return 0, &InvalidRequest{Field: controller.limitName(), Message: "limit must be a whole number"}
	}
	return limit, nil
}

func (controller *MembersController) authorizationHeader() string {
	return "Authorization"
}

func (controller *MembersController) limitName() string {
	return "limit"
}

func (controller *MembersController) queryName() string {
	return "query"
}

func (controller *MembersController) cursorName() string {
	return "cursor"
}
