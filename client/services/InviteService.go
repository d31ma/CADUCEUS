// InviteService shows the meeting invitation that the server imported
// from the open message and records the user's RSVP, which the server sends
// to the organizer as an iCalendar REPLY.
//
// @Service
type InviteService struct {
	// @Needs
	calendarClient            *CalendarClient
	applicationEventPublisher ApplicationEventPublisher

	Invite    CalendarInvite
	HasInvite bool
	Busy      bool

	token string
}

// @OnEvent({"replay":true})
func (service *InviteService) SessionChanged(changed SessionChanged) {
	service.token = changed.Token
}

// @OnEvent({"replay":true})
func (service *InviteService) MessageOpened(opened OpenMessageChanged) {
	service.show(opened.CalendarInvite)
}

func (service *InviteService) Respond(status string) {
	service.respond(status)
}

func (service *InviteService) show(invite *CalendarInvite) {
	service.HasInvite = invite != nil
	service.Invite = service.labelled(service.value(invite))
}

func (service *InviteService) value(invite *CalendarInvite) CalendarInvite {
	if invite == nil {
		return CalendarInvite{}
	}
	return *invite
}

func (service *InviteService) labelled(invite CalendarInvite) CalendarInvite {
	invite.DisplayStatus = service.displayStatus(invite)
	invite.StatusLabel = service.statusLabel(invite.Status == "cancelled", invite.DisplayStatus)
	invite.SenderResponseText = service.responseText(invite.SenderResponse)
	return invite
}

func (service *InviteService) displayStatus(invite CalendarInvite) string {
	return TextUtils{}.Or(TextUtils{}.Or(service.responseStatus(invite.SenderResponse), invite.MyStatus), "needsAction")
}

func (service *InviteService) responseStatus(response *InviteResponse) string {
	if response == nil {
		return ""
	}
	return response.Status
}

func (service *InviteService) statusLabel(cancelled bool, status string) string {
	labels := map[string]string{"accepted": "Going", "declined": "Not going", "tentative": "Maybe", "delegated": "Delegated", "completed": "Completed"}
	return TextUtils{}.Choose(cancelled, "Canceled", TextUtils{}.Or(labels[status], "No response"))
}

func (service *InviteService) responseText(response *InviteResponse) string {
	if response == nil {
		return ""
	}
	verbs := map[string]string{"accepted": "accepted", "declined": "declined", "tentative": "tentatively accepted", "delegated": "delegated", "completed": "completed"}
	return TextUtils{}.Or(TextUtils{}.Or(response.Name, response.Email), "The attendee") + TextUtils{}.Choose(verbs[response.Status] == "", " has not responded to", " "+verbs[response.Status]) + " this invitation."
}

func (service *InviteService) respond(status string) {
	if service.Invite.Id != "" && service.Invite.CanRespond && !service.Busy {
		service.send(status)
	}
}

func (service *InviteService) send(status string) {
	service.Busy = true
	_, failure := service.calendarClient.RespondToInvite(map[string]any{"eventId": service.Invite.Id, "status": status}, "Bearer "+service.token)
	service.responded(FailureUtils{}.Code(failure), status)
	service.Busy = false
}

func (service *InviteService) responded(code string, status string) {
	labels := map[string]string{"accepted": "Accepted.", "declined": "Declined.", "tentative": "Tentative."}
	switch code {
	case "":
		service.Invite.MyStatus = status
		service.Invite = service.labelled(service.Invite)
		service.toast(labels[status])
	case FailureUtils{}.OfflineCode():
		service.toast("Network error.")
	default:
		service.toast("Could not record your response.")
	}
}

func (service *InviteService) toast(message string) {
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: message}, false)
}
