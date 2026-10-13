import (
	"slices"
	"strings"
)

// SenderService owns the From identity and the signatures of the composer:
// it loads the mailbox's aliases, connected sending accounts, signatures and
// mail preferences, chooses the default From address and signature of a new
// message, and switches both when the person picks another identity. The
// message body stays in ComposeEditorService, so a signature change is a
// SignatureApplyRequested event for the editor.
//
// @Service
type SenderService struct {
	// @Needs
	usersClient *UsersClient
	// @Needs
	externalAccountsClient *ExternalAccountsClient
	// @Needs
	signaturesClient          *SignaturesClient
	applicationEventPublisher ApplicationEventPublisher

	From              string
	Options           []SenderOption
	Signatures        []MailSignature
	ActiveSignatureID string

	token       string
	owner       string
	addresses   []string
	preferences MailPreferences
	fetched     bool
	loaded      []ComposeLoaded
	started     bool
}

// @OnStart
func (sender *SenderService) Start() {
	sender.Options = []SenderOption{}
	sender.Signatures = []MailSignature{}
	sender.preferences = MailPreferenceUtils{}.Defaults()
}

// SessionChanged loads the identities and signatures of the mailbox.
//
// @OnEvent({"replay":true})
func (sender *SenderService) SessionChanged(changed SessionChanged) {
	sender.token = changed.Token
	sender.owner = strings.ToLower(changed.Email)
	sender.loadWhen(sender.pendingFetch(changed.SignedIn))
	sender.startWhenReady()
}

// Loaded keeps the opening state of the composer; a reloaded draft brings
// its own From address.
//
// @OnEvent({"replay":true})
func (sender *SenderService) Loaded(loaded ComposeLoaded) {
	sender.loaded = []ComposeLoaded{loaded}
	sender.followReload(loaded)
	sender.startWhenReady()
}

// Change switches the From identity and its default signature.
func (sender *SenderService) Change(address string) {
	sender.switchTo(strings.ToLower(address), sender.owns(strings.ToLower(address)))
}

// ChangeSignature selects a named signature or removes the active one.
func (sender *SenderService) ChangeSignature(id string) {
	sender.useSignature(sender.signatureNamed(id))
	sender.publish(SenderChosen{Reason: sender.signatureReason(), Address: sender.From, Preferences: sender.preferences})
}

// EditorSignatureChanged follows a signature change made in the editor, for
// example by a translation.
//
// @OnEvent
func (sender *SenderService) EditorSignatureChanged(changed EditorSignatureChanged) {
	sender.ActiveSignatureID = changed.SignatureID
}

// ManageSignatures opens settings, where signatures are managed.
func (sender *SenderService) ManageSignatures() {
	sender.publish(NavigationRequested{Target: "/settings"})
}

func (sender *SenderService) publish(value any) {
	sender.applicationEventPublisher.PublishEvent(value, false)
}

func (sender *SenderService) authorization() string {
	return "Bearer " + sender.token
}

func (SenderService) startReason() string { return "start" }

func (SenderService) signatureReason() string { return "signature" }

func (SenderService) reloadReason() string { return "reload" }

func (sender *SenderService) loadWhen(pending []bool) {
	for range pending {
		sender.load()
	}
}

func (sender *SenderService) pendingFetch(signedIn bool) []bool {
	if sender.fetched || !signedIn {
		return nil
	}
	return []bool{true}
}

func (sender *SenderService) load() {
	preferences, preferencesProblem := sender.usersClient.Preferences(sender.authorization())
	aliases, _ := sender.usersClient.Aliases(sender.authorization())
	external, _ := sender.externalAccountsClient.Accounts(sender.authorization())
	signatures, signaturesProblem := sender.signaturesClient.Signatures(sender.authorization())
	identities := ExternalAccountUtils{}.SenderIdentities(external.Accounts)
	primary := strings.ToLower(SenderService{}.firstOf(aliases.Email, sender.owner))
	sender.addresses = SenderService{}.addressesOf(primary, aliases.Aliases, identities)
	sender.Options = SenderService{}.optionsOf(sender.addresses, identities)
	sender.preferences = SenderService{}.preferencesOr(preferences.MailPreferences, preferencesProblem, sender.preferences)
	FlowUtils{}.When(HTTPUtils{}.Refused(preferencesProblem), func() { sender.publish(SessionCheckRequested{Reason: "compose"}) })()
	sender.Signatures = SenderService{}.signaturesOr(signatures, signaturesProblem)
	sender.From = primary
	sender.fetched = true
	sender.publish(EditorPreferences{RichText: sender.preferences.RichText, Spellcheck: sender.preferences.Spellcheck, SpellcheckLanguage: sender.preferences.SpellcheckLanguage})
}

func (SenderService) firstOf(value string, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func (SenderService) addressesOf(primary string, aliases []string, identities []SenderIdentity) []string {
	addresses := append([]string{primary}, aliases...)
	for _, identity := range identities {
		addresses = append(addresses, identity.Address)
	}
	return RecipientUtils{}.Split(RecipientUtils{}.MergeAutomatic(strings.Join(addresses, ","), nil, nil))
}

func (SenderService) optionsOf(addresses []string, identities []SenderIdentity) []SenderOption {
	options := []SenderOption{}
	for _, address := range addresses {
		options = append(options, SenderOption{Address: address, Label: SenderService{}.labelOf(address, identities)})
	}
	return options
}

func (SenderService) labelOf(address string, identities []SenderIdentity) string {
	labels := []string{}
	for _, identity := range identities {
		labels = append(labels, SenderService{}.identityLabel(address, identity)...)
	}
	return append(labels, address)[0]
}

func (SenderService) identityLabel(address string, identity SenderIdentity) []string {
	if identity.Address == address {
		return []string{address + " — " + identity.Label}
	}
	return nil
}

func (SenderService) preferencesOr(preferences *MailPreferences, problem error, fallback MailPreferences) MailPreferences {
	if problem == nil {
		return MailPreferenceUtils{}.Normalize(preferences)
	}
	return fallback
}

func (SenderService) signaturesOr(signatures []MailSignature, problem error) []MailSignature {
	if problem == nil {
		return SignatureUtils{}.SanitizeAll(signatures)
	}
	return []MailSignature{}
}

func (SenderService) ownedIn(addresses []string, address string) string {
	return strings.Join(slices.DeleteFunc([]string{address}, func(candidate string) bool { return !slices.Contains(addresses, candidate) }), "")
}

func (sender *SenderService) owns(address string) bool {
	return slices.Contains(sender.addresses, address) && address != ""
}

func (sender *SenderService) startWhenReady() {
	for _, loaded := range sender.pendingStart() {
		sender.start(loaded)
	}
}

func (sender *SenderService) pendingStart() []ComposeLoaded {
	if sender.started || !sender.fetched {
		return nil
	}
	return sender.loaded
}

// start chooses the From address and signature of the opening message, like
// initFromPrefill: a new message takes the default From address and its
// automatic recipients, and every new message, reply or forward gets the
// default signature before quoted content.
func (sender *SenderService) start(loaded ComposeLoaded) {
	fresh := SenderService{}.isNew(loaded)
	sender.started = true
	sender.From = sender.ownedOr(strings.ToLower(SenderService{}.firstOf(loaded.From, sender.owner)), sender.From)
	sender.From = sender.defaultFrom(fresh, loaded.From, sender.preferences.DefaultFrom)
	sender.ActiveSignatureID = loaded.SignatureID
	sender.defaultSignature(SenderService{}.wantsSignature(loaded))
	sender.publish(sender.chosen(sender.startReason(), fresh))
}

func (SenderService) isNew(loaded ComposeLoaded) bool {
	if loaded.DraftID == "" && loaded.ScheduledID == "" && !loaded.Recovered {
		return true
	}
	return false
}

func (SenderService) wantsSignature(loaded ComposeLoaded) bool {
	if loaded.DraftID == "" && loaded.ScheduledID == "" && loaded.SignatureID == "" {
		return true
	}
	return false
}

func (sender *SenderService) ownedOr(address string, fallback string) string {
	if slices.Contains(sender.addresses, address) {
		return address
	}
	return fallback
}

func (sender *SenderService) defaultFrom(fresh bool, explicit string, preferred string) string {
	if fresh && explicit == "" && preferred != "" && slices.Contains(sender.addresses, preferred) {
		return preferred
	}
	return sender.From
}

func (sender *SenderService) defaultSignature(applies bool) {
	if applies {
		sender.useSignature(SignatureUtils{}.DefaultFor(sender.Signatures, sender.From))
	}
}

func (sender *SenderService) useSignature(signature MailSignature) {
	sender.ActiveSignatureID = signature.ID
	sender.publish(SignatureApplyRequested{SignatureID: signature.ID, Text: signature.Text})
}

func (sender *SenderService) chosen(reason string, apply bool) SenderChosen {
	return SenderChosen{
		Reason:      reason,
		Address:     sender.From,
		Apply:       apply,
		NextCc:      append([]string{}, sender.preferences.AutoCcBySender[strings.ToLower(sender.From)]...),
		NextBcc:     append([]string{}, sender.preferences.AutoBccBySender[strings.ToLower(sender.From)]...),
		Preferences: sender.preferences,
	}
}

func (sender *SenderService) followReload(loaded ComposeLoaded) {
	if sender.started && loaded.Reason == sender.reloadReason() {
		sender.From = SenderService{}.firstOf(SenderService{}.ownedIn(sender.addresses, strings.ToLower(loaded.From)), sender.From)
		sender.ActiveSignatureID = loaded.SignatureID
		sender.publish(sender.chosen(sender.reloadReason(), false))
	}
}

func (sender *SenderService) switchTo(address string, owned bool) {
	if !owned {
		return
	}
	sender.From = address
	sender.useSignature(SignatureUtils{}.DefaultFor(sender.Signatures, address))
	sender.publish(sender.chosen("change", true))
}

func (sender *SenderService) signatureNamed(id string) MailSignature {
	found := MailSignature{}
	for _, signature := range sender.Signatures {
		found = SenderService{}.namedOr(signature, id, found)
	}
	return found
}

func (SenderService) namedOr(signature MailSignature, id string, found MailSignature) MailSignature {
	if signature.ID == id && id != "" {
		return signature
	}
	return found
}
