import "net/http"

// ActivityRoute answers GET /activity: the opens and clicks of the mail the
// caller's own addresses sent.
type ActivityRoute struct {
	Parts    MailboxParts
	Users    *UsersHelper
	Tracking *TrackingHelper
}

// Feed answers the activity feed and analytics.
func (route ActivityRoute) Feed(request *http.Request) *http.Response {
	caller, refusal := route.Parts.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	options, err := ActivityQuery{}.Options(MailboxQuery{}.Of(request), EpochTime{}.Now())
	if err != nil {
		return Answer{}.BadRequest(err.Error())
	}
	activity, err := route.activity(caller, options)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	return Answer{}.Ok(activity)
}

func (route ActivityRoute) activity(caller RouteCaller, options TrackingActivityOptions) (map[string]any, error) {
	senders, err := route.ownedSenders(caller.Email)
	if err != nil {
		return nil, err
	}
	mappings, err := route.Parts.Mappings(caller)
	if err != nil {
		return nil, err
	}
	options.AllowedDomains = caller.Domains
	options.OwnerEmail = caller.Email
	options.OwnedSenders = senders
	options.SentFolders = mappings.FolderFilter(caller.Domains, "sent")
	options.DraftFolders = mappings.FolderFilter(caller.Domains, "drafts")
	for _, folders := range []*map[string]string{&options.SentFolders, &options.DraftFolders} {
		if *folders == nil {
			*folders = map[string]string{}
		}
	}
	return route.Tracking.Activity(options)
}

// ownedSenders answers the exact addresses the mailbox may send as: its
// primary address and aliases (only the address itself without a user).
func (route ActivityRoute) ownedSenders(ownerEmail string) ([]string, error) {
	security := Security{}
	owner := security.NormalizeEmailAddress(ownerEmail)
	if owner == "" {
		return []string{}, nil
	}
	_, user, err := route.Users.FindByEmail(owner)
	if err != nil || user == nil {
		return []string{owner}, err
	}
	aliases, _ := user["aliases"].([]string)
	addresses := []string{}
	for _, address := range append([]string{(TextValue{}).Of(user["email"])}, aliases...) {
		if normalized := security.NormalizeEmailAddress(address); normalized != "" {
			addresses = append(addresses, normalized)
		}
	}
	return MailAddresses{}.Unique(addresses), nil
}
