package sending

import (
	"time"

	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
)

type Sender struct {
	stores core.Stores
	ports  core.Ports
}

func NewSender(stores core.Stores, ports core.Ports) Sender {
	return Sender{stores: stores, ports: ports}
}

func (sender Sender) Send(caller oidc.Caller, authorization string, request core.SendRequest) (core.SendOutcome, error) {
	normalized := RequestCheck{}.Normalized(request)
	err := RequestCheck{}.Check(normalized)
	if err != nil {
		return core.SendOutcome{}, err
	}
	sendKey := Hashes{}.sendKey(caller.MemberId, normalized.ClientMessageId)
	earlier, found, err := NewPreviousSend(sender.stores.Messages).find(caller.DomainId, sendKey, sender.ports.Now())
	if err != nil {
		return core.SendOutcome{}, err
	}
	draft := Draft{Request: normalized, SendKey: sendKey}
	return sender.route(found)(caller, authorization, draft, earlier)
}

func (sender Sender) route(found bool) func(oidc.Caller, string, Draft, core.Message) (core.SendOutcome, error) {
	if found {
		return sender.again
	}
	return sender.fresh
}

func (sender Sender) again(caller oidc.Caller, authorization string, draft Draft, earlier core.Message) (core.SendOutcome, error) {
	completed, err := NewRetry(sender.stores, sender.ports.Stager).complete(earlier)
	if err != nil {
		return core.SendOutcome{}, err
	}
	return Outcome{}.of(completed)
}

func (sender Sender) fresh(caller oidc.Caller, authorization string, draft Draft, earlier core.Message) (core.SendOutcome, error) {
	attachments, err := NewAttachmentCheck(sender.ports.Attachments).refs(caller.DomainId, draft.Request.AttachmentIds)
	if err != nil {
		return core.SendOutcome{}, err
	}
	draft.Attachments = attachments
	now := sender.ports.Now()
	placed, err := sender.place(caller, authorization, draft, now)
	if err != nil {
		return core.SendOutcome{}, err
	}
	composed, err := sender.compose(caller, placed, now)
	if err != nil {
		return core.SendOutcome{}, err
	}
	planned := WriteBudget{}.planned(composed)
	err = WriteBudget{}.check(planned)
	if err != nil {
		return core.SendOutcome{}, err
	}
	message, err := NewMessageWriter(sender.stores, sender.ports.Stager, now).Write(caller, composed, WriteBudget{}.remaining(planned))
	if err != nil {
		return core.SendOutcome{}, err
	}
	return Outcome{}.of(message)
}

func (sender Sender) place(caller oidc.Caller, authorization string, draft Draft, now time.Time) (Draft, error) {
	chosen, err := NewRecipientCheck(sender.ports.Directory).Checked(authorization, draft.Request)
	if err != nil {
		return draft, err
	}
	placement, err := NewConversationPicker(sender.stores, now).Pick(caller, draft.Request, chosen)
	if err != nil {
		return draft, err
	}
	thread, err := NewThreadRoot(sender.stores.Messages).Find(caller, core.ConversationIds{}.Wire(placement.Conversation), draft.Request.ThreadRootId)
	if err != nil {
		return draft, err
	}
	draft.Placement = placement
	draft.Thread = thread
	draft.Rows = DeliveryPlan{}.rows(placement, chosen, draft.Request.Email.SendToExternal)
	return draft, nil
}

func (sender Sender) compose(caller oidc.Caller, draft Draft, now time.Time) (Draft, error) {
	composed, err := NewEmailHeaders(sender.stores, sender.ports.Directory, now).Build(caller, draft)
	if err != nil {
		return draft, err
	}
	rich, err := sender.ports.Renderer.Markdown(composed.Request.Body)
	if err != nil {
		return draft, Failures{}.renderer(err)
	}
	err = core.MessageBudget{}.Check(composed.Request.Body, core.MessageBudget{}.BlocksJson(rich.Blocks))
	if err != nil {
		return draft, err
	}
	mentioned, err := NewMentions(sender.stores.Members).of(caller.DomainId, composed.Request.Body, Readers{}.of(composed.Placement.all()), caller.MemberId)
	if err != nil {
		return draft, err
	}
	composed.Rich = rich
	composed.Mentions = mentioned
	return composed, nil
}
