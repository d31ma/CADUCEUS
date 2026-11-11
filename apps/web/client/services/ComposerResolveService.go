import "slices"

// @Service
type ComposerResolveService struct {
	// @Needs
	directory *DirectoryClient
	// @Needs
	auth *ComposerAuthService
	// @Needs
	words *ComposerWordsService
}

func (resolve *ComposerResolveService) Resolve(addresses []string) ([]ResolvedRecipient, error) {
	authorization, failure := resolve.auth.Authorization()
	answer, failure := resolve.answerUnlessFailed(authorization, addresses, failure)
	return answer.Recipients, failure
}

func (resolve *ComposerResolveService) RecipientOf(resolved ResolvedRecipient, role string, delivery string) Recipient {
	displayName := resolve.nameOf(resolved)
	recipient := Recipient{
		Address:     resolved.Address,
		DisplayName: displayName,
		FirstName:   resolve.words.FirstName(displayName),
		Initials:    resolve.words.Initials(displayName),
		Kind:        resolved.Kind,
		Role:        role,
		Delivery:    resolve.deliveryOf(resolved, delivery),
		State:       "ready",
		ChatAllowed: slices.Contains(resolved.AllowedDeliveries, "chat"),
		ChatReason:  resolve.words.ChatReason(resolved.Kind, resolved.Address, resolve.auth.DomainId()),
		BothReason:  resolve.words.BothReason(resolved.Kind),
	}
	return resolve.labelled(recipient)
}

func (resolve *ComposerResolveService) Checking(address string, role string) Recipient {
	return resolve.labelled(Recipient{Address: address, DisplayName: address, Initials: resolve.words.Initials(address), Role: role, State: "checking"})
}

func (resolve *ComposerResolveService) Invalid(address string, role string) Recipient {
	return resolve.labelled(Recipient{Address: address, DisplayName: address, Initials: resolve.words.Initials(address), Role: role, State: "invalid"})
}

func (resolve *ComposerResolveService) Labelled(recipient Recipient) Recipient {
	return resolve.labelled(recipient)
}

func (resolve *ComposerResolveService) labelled(recipient Recipient) Recipient {
	recipient.Label = resolve.words.ChipLabel(recipient)
	recipient.AriaLabel = resolve.words.ChipAriaLabel(recipient)
	return recipient
}

func (resolve *ComposerResolveService) answerUnlessFailed(authorization string, addresses []string, failure error) (ResolveResponse, error) {
	if failure != nil {
		return ResolveResponse{}, failure
	}
	return BehaviorsAnswer[ResolveResponse](resolve.directory.Resolve(authorization, ResolveRequest{Addresses: addresses}))
}

func (resolve *ComposerResolveService) nameOf(resolved ResolvedRecipient) string {
	if resolved.DisplayName == "" {
		return resolved.Address
	}
	return resolved.DisplayName
}

func (resolve *ComposerResolveService) deliveryOf(resolved ResolvedRecipient, wanted string) string {
	if slices.Contains(resolved.AllowedDeliveries, wanted) {
		return wanted
	}
	return resolved.DefaultDelivery
}
