import (
	"slices"
	"strings"
)

// @Service
type ChannelAddressService struct {
	Open             bool
	ChannelId        string
	Title            string
	Domain           string
	LocalPart        string
	CurrentLocalPart string
	Editing          bool
	Busy             bool
	Problem          string
	CurrentText      string
	Help             string
	MailText         string
	// @Needs
	channelList *ChannelListService
	// @Needs
	session *AdminSessionService
	// @Needs
	channelsClient *ChannelsClient
	// @Needs
	problems *AdminProblemService
}

func (dialog *ChannelAddressService) Edit(channelId string) {
	channel := dialog.channelList.Find(channelId)
	label := "#" + channel.Name
	dialog.ChannelId = channel.ChannelId
	dialog.Domain = dialog.channelList.Domain
	dialog.LocalPart = dialog.localPartOf(channel.EmailAddress)
	dialog.CurrentLocalPart = dialog.LocalPart
	dialog.Editing = channel.EmailAddress != ""
	dialog.Title = "Email address for " + label
	dialog.CurrentText = "This is the current address of " + label + dialog.period()
	dialog.Help = "Ends with @" + dialog.Domain + ". The address must not belong to a person, an alias or another channel."
	dialog.MailText = "Mail sent to this address shows in " + label + " as an email conversation. Members reply as email. Chat in " + label + " stays inside " + dialog.Domain + dialog.period()
	dialog.Problem = ""
	dialog.Busy = false
	dialog.Open = true
}

func (dialog *ChannelAddressService) Cancel() {
	dialog.Open = false
}

func (dialog *ChannelAddressService) Save() {
	dialog.Problem = dialog.localPartProblem(dialog.LocalPart)
	dialog.sendWhenValid(dialog.Problem)
}

func (dialog *ChannelAddressService) Remove() {
	removed := ""
	dialog.update(UpdateChannelRequest{EmailAddress: &removed})
}

func (dialog *ChannelAddressService) sendWhenValid(problem string) {
	if problem != "" {
		return
	}
	dialog.send(dialog.LocalPart + dialog.atSign() + dialog.Domain)
}

func (dialog *ChannelAddressService) send(address string) {
	dialog.update(UpdateChannelRequest{EmailAddress: &address})
}

func (dialog *ChannelAddressService) update(request UpdateChannelRequest) {
	dialog.Busy = true
	answer, failure := dialog.channelsClient.UpdateChannel(dialog.session.Authorization(), dialog.ChannelId, request)
	dialog.Busy = false
	dialog.Problem = dialog.problemText(dialog.problems.Of(failure))
	dialog.closeWhenSaved(answer, failure)
}

func (dialog *ChannelAddressService) closeWhenSaved(answer Channel, failure error) {
	if failure != nil {
		return
	}
	dialog.channelList.Replace(answer)
	dialog.Open = false
}

func (dialog *ChannelAddressService) localPartOf(address string) string {
	localPart, _, _ := strings.Cut(address, dialog.atSign())
	return localPart
}

func (dialog *ChannelAddressService) localPartProblem(localPart string) string {
	checks := append(dialog.localPartChecks(localPart), ChannelAddressCheck{Failed: true})
	return checks[slices.IndexFunc(checks, dialog.failed)].Message
}

func (dialog *ChannelAddressService) localPartChecks(localPart string) []ChannelAddressCheck {
	return []ChannelAddressCheck{
		{Failed: localPart == "", Message: "Enter the part of the address before @" + dialog.Domain + dialog.period()},
		{Failed: len(localPart) > dialog.longestLocalPart(), Message: "Use at most 64 characters before the @."},
		{Failed: strings.Trim(localPart, dialog.allowedCharacters()) != "", Message: "Use only lowercase letters, digits, dots, hyphens and underscores."},
		{Failed: strings.Trim(localPart, dialog.period()) != localPart, Message: "The address cannot start or end with a dot."},
	}
}

func (dialog *ChannelAddressService) failed(check ChannelAddressCheck) bool {
	return check.Failed
}

func (dialog *ChannelAddressService) problemText(problem string) string {
	text, known := dialog.problemTexts()[problem]
	if !known {
		return "Mesail could not reach the server. Check your connection, then try again."
	}
	return text
}

func (dialog *ChannelAddressService) problemTexts() map[string]string {
	return map[string]string{
		"":          "",
		"conflict":  "This address already belongs to a member, alias or another channel.",
		"forbidden": "Only a domain admin can change channel addresses.",
		"notFound":  "This channel no longer exists.",
		"invalid":   "The server refused this address. Check it, then try again.",
		"signedOut": "Your session ended. Sign in again, then try again.",
	}
}

func (dialog *ChannelAddressService) longestLocalPart() int {
	return 64
}

func (dialog *ChannelAddressService) allowedCharacters() string {
	return "abcdefghijklmnopqrstuvwxyz0123456789._-"
}

func (dialog *ChannelAddressService) atSign() string {
	return "@"
}

func (dialog *ChannelAddressService) period() string {
	return "."
}
