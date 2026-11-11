package channels

import (
	"errors"
	"time"

	"mesail.dev/packages/oidc"
)

type Workflow struct {
	ports  Ports
	store  Store
	access Access
	events Events
}

func NewWorkflow(ports Ports) Workflow {
	store := NewStore(ports.Documents, ports.Relay)
	return Workflow{ports: ports, store: store, access: NewAccess(store), events: NewEvents(ports.Settings, ports.Now)}
}

func (workflow Workflow) List(authorization string) ([]ChannelDocument, error) {
	caller, err := workflow.ports.Callers.Caller(authorization)
	if err != nil {
		return nil, err
	}
	return workflow.access.Listed(caller)
}

func (workflow Workflow) Create(authorization string, request CreateRequest) (ChannelDocument, error) {
	caller, err := workflow.ports.Callers.Caller(authorization)
	if err != nil {
		return ChannelDocument{}, err
	}
	err = Rules{}.CheckCreate(request)
	if err != nil {
		return ChannelDocument{}, err
	}
	taken, err := workflow.store.ChannelNamed(caller.DomainId, request.Name)
	if err != nil {
		return ChannelDocument{}, err
	}
	err = workflow.nameRefusal(taken, request.Name)
	if err != nil {
		return ChannelDocument{}, err
	}
	err = workflow.events.Ready()
	if err != nil {
		return ChannelDocument{}, err
	}
	return workflow.created(workflow.newChannel(caller.DomainId, caller.MemberId, request))
}

func (workflow Workflow) Join(authorization string, channelId string) error {
	caller, err := workflow.ports.Callers.Caller(authorization)
	if err != nil {
		return err
	}
	view, err := workflow.writableView(caller, channelId)
	if err != nil {
		return err
	}
	return workflow.add(view.Document, caller.MemberId)
}

func (workflow Workflow) Leave(authorization string, channelId string) error {
	caller, err := workflow.ports.Callers.Caller(authorization)
	if err != nil {
		return err
	}
	view, err := workflow.access.Visible(caller, channelId)
	if err != nil {
		return err
	}
	membership, joined, err := workflow.store.FindMembership(caller.DomainId, view.Document.Id, caller.MemberId)
	if err != nil {
		return err
	}
	return workflow.remove(view.Document, membership, joined)
}

func (workflow Workflow) Invite(authorization string, channelId string, memberIds []string) error {
	caller, err := workflow.ports.Callers.Caller(authorization)
	if err != nil {
		return err
	}
	err = Rules{}.CheckInvite(memberIds)
	if err != nil {
		return err
	}
	view, err := workflow.writableView(caller, channelId)
	if err != nil {
		return err
	}
	err = workflow.checkMembers(authorization, memberIds)
	if err != nil {
		return err
	}
	return workflow.addAll(view.Document, memberIds)
}

func (workflow Workflow) Update(authorization string, channelId string, request UpdateRequest) (ChannelDocument, error) {
	caller, err := workflow.ports.Callers.Caller(authorization)
	if err != nil {
		return ChannelDocument{}, err
	}
	view, err := workflow.access.Visible(caller, channelId)
	if err != nil {
		return ChannelDocument{}, err
	}
	err = Rules{}.CheckUpdate(caller, view, request)
	if err != nil {
		return ChannelDocument{}, err
	}
	change, err := Rules{}.AddressChange(caller.DomainId, view.Document.EmailAddress, request.EmailAddress)
	if err != nil {
		return ChannelDocument{}, err
	}
	document, err := workflow.archived(workflow.applied(view.Document, change, request.Topic), request.Archived)
	if err != nil {
		return ChannelDocument{}, err
	}
	err = workflow.moveAddress(authorization, change, channelId)
	if err != nil {
		return ChannelDocument{}, err
	}
	return document, workflow.store.SaveChannel(document)
}

func (workflow Workflow) writableView(caller oidc.Caller, channelId string) (ChannelView, error) {
	view, err := workflow.access.Visible(caller, channelId)
	if err != nil {
		return ChannelView{}, err
	}
	return view, Rules{}.CheckWritable(view.Document)
}

func (workflow Workflow) created(document ChannelDocument) (ChannelDocument, error) {
	id, err := workflow.store.InsertChannel(document)
	if err != nil {
		return ChannelDocument{}, err
	}
	document.Id = id
	document, err = workflow.events.Announce(document)
	if err != nil {
		return ChannelDocument{}, err
	}
	err = workflow.store.SaveChannel(document)
	if err != nil {
		return ChannelDocument{}, err
	}
	return document, workflow.add(document, document.CreatedBy)
}

func (workflow Workflow) add(document ChannelDocument, memberId string) error {
	membership, joined, err := workflow.store.FindMembership(document.DomainId, document.Id, memberId)
	if err != nil {
		return err
	}
	return workflow.rejoin(document, membership, joined, memberId)
}

func (workflow Workflow) rejoin(document ChannelDocument, membership MembershipDocument, joined bool, memberId string) error {
	switch {
	case !joined:
		return workflow.insertAdded(document, workflow.newMembership(document, memberId))
	case membership.Left:
		membership.Left = false
		membership.JoinedAt = workflow.now()
		return workflow.saveAdded(document, membership)
	}
	return nil
}

func (workflow Workflow) insertAdded(document ChannelDocument, membership MembershipDocument) error {
	membership, err := workflow.events.MemberAdded(document, membership)
	if err != nil {
		return err
	}
	return workflow.store.InsertMembership(membership)
}

func (workflow Workflow) saveAdded(document ChannelDocument, membership MembershipDocument) error {
	membership, err := workflow.events.MemberAdded(document, membership)
	if err != nil {
		return err
	}
	return workflow.store.SaveMembership(membership)
}

func (workflow Workflow) remove(document ChannelDocument, membership MembershipDocument, joined bool) error {
	if !joined || membership.Left {
		return nil
	}
	membership.Left = true
	return workflow.saveRemoved(document, membership)
}

func (workflow Workflow) saveRemoved(document ChannelDocument, membership MembershipDocument) error {
	membership, err := workflow.events.MemberRemoved(document, membership)
	if err != nil {
		return err
	}
	return workflow.store.SaveMembership(membership)
}

func (workflow Workflow) checkMembers(authorization string, memberIds []string) error {
	failures := []error{}
	for _, memberId := range memberIds {
		found, err := workflow.ports.Directory.MemberInDomain(authorization, memberId)
		failures = append(failures, workflow.memberRefusal(found, err))
	}
	return errors.Join(failures...)
}

func (workflow Workflow) memberRefusal(found bool, failure error) error {
	switch {
	case failure != nil:
		return &Unavailable{Message: workflow.directoryDown(), Cause: failure}
	case found:
		return nil
	}
	return &NotFound{Message: "no such member in this domain"}
}

func (workflow Workflow) addAll(document ChannelDocument, memberIds []string) error {
	failures := []error{}
	for _, memberId := range memberIds {
		failures = append(failures, workflow.add(document, memberId))
	}
	return errors.Join(failures...)
}

func (workflow Workflow) moveAddress(authorization string, change AddressChange, channelId string) error {
	err := workflow.claim(authorization, change, channelId)
	if err != nil {
		return err
	}
	return workflow.release(authorization, change)
}

func (workflow Workflow) claim(authorization string, change AddressChange, channelId string) error {
	if change.New == "" || change.New == change.Old {
		return nil
	}
	return workflow.claimRefusal(workflow.ports.Directory.ClaimChannelAddress(authorization, change.New, channelId))
}

func (workflow Workflow) release(authorization string, change AddressChange) error {
	if change.Old == "" || change.Old == change.New {
		return nil
	}
	return workflow.unavailable(workflow.ports.Directory.ReleaseChannelAddress(authorization, change.Old))
}

func (workflow Workflow) claimRefusal(failure error) error {
	switch {
	case failure == nil:
		return nil
	case errors.As(failure, &AddressTaken{}):
		return &Duplicate{Message: failure.Error()}
	case errors.As(failure, &AddressRefused{}):
		return &InvalidRequest{Message: failure.Error(), Field: "emailAddress"}
	}
	return &Unavailable{Message: workflow.directoryDown(), Cause: failure}
}

func (workflow Workflow) unavailable(failure error) error {
	if failure == nil {
		return nil
	}
	return &Unavailable{Message: workflow.directoryDown(), Cause: failure}
}

func (Workflow) applied(document ChannelDocument, change AddressChange, topic *string) ChannelDocument {
	document.EmailAddress = change.New
	if topic != nil {
		document.Topic = *topic
	}
	return document
}

func (workflow Workflow) archived(document ChannelDocument, archived *bool) (ChannelDocument, error) {
	switch {
	case archived == nil, *archived == document.Archived:
		return document, nil
	case *archived:
		document.Archived = true
		return workflow.events.Archived(document)
	}
	document.Archived = false
	return document, nil
}

func (workflow Workflow) nameRefusal(taken bool, name string) error {
	if taken {
		return &Conflict{Message: "the channel name " + name + " is already used in this domain"}
	}
	return nil
}

func (workflow Workflow) newChannel(domainId string, memberId string, request CreateRequest) ChannelDocument {
	return ChannelDocument{
		DomainId:      domainId,
		Name:          request.Name,
		Topic:         request.Topic,
		Visibility:    request.Visibility,
		CreatedBy:     memberId,
		CreatedAt:     workflow.now(),
		PendingEvents: []string{},
		DeadEvents:    []string{},
	}
}

func (workflow Workflow) newMembership(document ChannelDocument, memberId string) MembershipDocument {
	return MembershipDocument{
		DomainId:      document.DomainId,
		ChannelId:     document.Id,
		MemberId:      memberId,
		JoinedAt:      workflow.now(),
		PendingEvents: []string{},
		DeadEvents:    []string{},
	}
}

func (workflow Workflow) now() string {
	return workflow.ports.Now().UTC().Format(time.RFC3339)
}

func (Workflow) directoryDown() string {
	return "the directory is not available"
}
