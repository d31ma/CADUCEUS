package channels

import "mesail.dev/packages/oidc"

type Access struct {
	store Store
}

func NewAccess(store Store) Access {
	return Access{store: store}
}

func (access Access) Visible(caller oidc.Caller, channelId string) (ChannelView, error) {
	storedId, err := WireIds{}.StoredId(channelId)
	if err != nil {
		return ChannelView{}, err
	}
	document, found, err := access.store.FindChannel(caller.DomainId, storedId)
	if err != nil {
		return ChannelView{}, err
	}
	membership, joined, err := access.store.FindMembership(caller.DomainId, storedId, caller.MemberId)
	if err != nil {
		return ChannelView{}, err
	}
	view := ChannelView{Document: document, Member: access.active(membership, joined)}
	return view, access.hidden(found, view)
}

func (access Access) Listed(caller oidc.Caller) ([]ChannelDocument, error) {
	channels, err := access.store.Channels(caller.DomainId)
	if err != nil {
		return nil, err
	}
	memberships, err := access.store.MembershipsOf(caller.DomainId, caller.MemberId)
	return access.visibleOnes(channels, access.joinedIds(memberships)), err
}

func (Access) active(membership MembershipDocument, joined bool) bool {
	return joined && !membership.Left
}

func (Access) hidden(found bool, view ChannelView) error {
	if found && (view.Document.Visibility == (Rules{}).public() || view.Member) {
		return nil
	}
	return &NotFound{Message: "no such channel"}
}

func (Access) joinedIds(memberships []MembershipDocument) map[string]bool {
	joined := map[string]bool{}
	for _, membership := range memberships {
		joined[membership.ChannelId] = !membership.Left
	}
	return joined
}

func (access Access) visibleOnes(channels []ChannelDocument, joined map[string]bool) []ChannelDocument {
	visible := []ChannelDocument{}
	for _, document := range channels {
		visible = append(visible, access.ifVisible(ChannelView{Document: document, Member: joined[document.Id]})...)
	}
	return visible
}

func (access Access) ifVisible(view ChannelView) []ChannelDocument {
	if access.hidden(true, view) == nil {
		return []ChannelDocument{view.Document}
	}
	return nil
}
