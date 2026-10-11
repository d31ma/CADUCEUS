import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
)

// DraftUtils reads how the composer opens and writes its autosave snapshots,
// like initFromPrefill and the draft snapshot of the former compose view.
type DraftUtils struct{}

// Prefill merges the prefill another view stored for the composer (JSON text
// from StoredValues, read before the shell removed it) with the fields of the
// compose launch. The shell's first ComposeLaunched can come from the address
// before the shell has read storage, so the composer reads the key too.
func (DraftUtils) Prefill(stored string, launched map[string]any) ComposePrefill {
	fields := map[string]any{}
	json.Unmarshal([]byte(stored), &fields)
	maps.Copy(fields, launched)
	encoded, _ := json.Marshal(fields)
	prefill := ComposePrefill{}
	json.Unmarshal(encoded, &prefill)
	return prefill
}

// Pointer reads the draft this tab was writing before a reload.
func (DraftUtils) Pointer(stored string) DraftPointer {
	pointer := DraftPointer{}
	json.Unmarshal([]byte(stored), &pointer)
	return pointer
}

// Stored is the storage write that remembers the draft this tab writes.
func (DraftUtils) Stored(pointer DraftPointer) StorageWritten {
	encoded, _ := json.Marshal(pointer)
	return StorageWritten{Area: DraftUtils{}.names().Session, Values: map[string]string{DraftUtils{}.names().Pointer: string(encoded)}, Removed: []string{}}
}

// Forgotten is the storage write once the draft is sent or discarded.
func (DraftUtils) Forgotten() StorageWritten {
	return StorageWritten{Area: DraftUtils{}.names().Session, Values: map[string]string{}, Removed: []string{DraftUtils{}.names().Pointer}}
}

// Opening names how the composer opens: a draft the prefill names, the
// prefill itself, the draft this tab was writing, or a blank message.
func (DraftUtils) Opening(prefill ComposePrefill, pointer DraftPointer) string {
	return DraftUtils{}.opening(prefill.DraftID, DraftUtils{}.explicit(prefill), pointer.DraftID)
}

// Snapshot is the complete autosave body of the message being written.
func (DraftUtils) Snapshot(fields MessageFields, clientID string, version int) DraftInput {
	return DraftInput{
		ClientID: clientID, Version: version, From: fields.From,
		To: RecipientUtils{}.Split(fields.To), Cc: RecipientUtils{}.Split(fields.Cc), Bcc: RecipientUtils{}.Split(fields.Bcc), ReplyTo: RecipientUtils{}.Split(fields.ReplyTo),
		Subject: fields.Subject, Text: fields.Text, HTML: fields.HTML, InReplyTo: fields.InReplyTo, References: append([]string{}, fields.References...),
		SourceEmailID: fields.SourceEmailID, Attachments: append([]ComposeAttachment{}, fields.Attachments...),
	}
}

// Fingerprint identifies a snapshot's content, whatever its version and the
// client identifier the server answered.
func (DraftUtils) Fingerprint(snapshot DraftInput) string {
	snapshot.Version, snapshot.ClientID = 0, ""
	encoded, _ := json.Marshal(snapshot)
	return string(encoded)
}

// Worth says whether a snapshot is worth a request: it changed since the last
// save, and it holds something or replaces a saved draft.
func (DraftUtils) Worth(snapshot DraftInput, saved string, draftID string) bool {
	kept := slices.Contains([]bool{DraftUtils{}.meaningful(snapshot), draftID != ""}, true)
	return !slices.Contains([]bool{DraftUtils{}.Fingerprint(snapshot) != saved, kept}, false)
}

// Fields are the header fields of a saved draft.
func (DraftUtils) Fields(draft Draft) MessageFields {
	return MessageFields{
		From: draft.Sender, To: RecipientUtils{}.Join(draft.To), Cc: RecipientUtils{}.Join(draft.Cc), Bcc: RecipientUtils{}.Join(draft.Bcc),
		ReplyTo: strings.Join(append(draft.ReplyTo, "")[:1], ""), Subject: draft.Subject, HTML: TextUtils{}.FirstOf(draft.HTMLBody, draft.Body), Text: draft.Body,
		InReplyTo: draft.InReplyTo, References: append([]string{}, draft.References...), SourceEmailID: draft.SourceEmailID, Attachments: append([]ComposeAttachment{}, draft.Attachments...),
	}
}

// PrefillFields are the header fields of a prefill.
func (DraftUtils) PrefillFields(prefill ComposePrefill) MessageFields {
	return MessageFields{
		From: prefill.From, To: prefill.To, Cc: prefill.Cc, Bcc: prefill.Bcc, ReplyTo: prefill.ReplyTo, Subject: prefill.Subject, HTML: prefill.Body,
		InReplyTo: prefill.InReplyTo, References: append([]string{}, prefill.References...), SourceEmailID: prefill.SourceEmailID,
		Attachments: append([]ComposeAttachment{}, prefill.Attachments...), OpenTracking: prefill.OpenTracking, LinkTracking: prefill.LinkTracking, RemindAt: prefill.RemindAt,
	}
}

// Meaningful says whether the opening message already holds something to save.
func (DraftUtils) Meaningful(fields MessageFields) bool {
	return DraftUtils{}.meaningful(DraftUtils{}.Snapshot(fields, "", 0))
}

func (DraftUtils) explicit(prefill ComposePrefill) bool {
	values := append([]string{prefill.ScheduledID, prefill.SourceEmailID, prefill.From, prefill.To, prefill.Cc, prefill.Bcc, prefill.ReplyTo, prefill.Subject, prefill.Body, prefill.InReplyTo}, prefill.References...)
	return strings.Join(values, "") != ""
}

func (DraftUtils) opening(draftID string, explicit bool, pointed string) string {
	switch {
	case draftID != "":
		return "draft"
	case explicit:
		return "prefill"
	case pointed != "":
		return "recovered"
	}
	return "blank"
}

func (DraftUtils) meaningful(snapshot DraftInput) bool {
	headers := strings.Join(append(append(append(snapshot.To, snapshot.Cc...), snapshot.ReplyTo...), snapshot.Subject, snapshot.Text, snapshot.HTML), "")
	return strings.TrimSpace(headers) != "" || len(snapshot.Attachments) > 0
}

func (DraftUtils) names() ComposeNames {
	return ComposeNames{Session: "session", Pointer: "caduceus_compose_draft"}
}
