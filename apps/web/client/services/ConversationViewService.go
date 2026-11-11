import (
	"cmp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// @Service
type ConversationViewService struct {
	// @Needs
	times *ConversationTimeService
	// @Needs
	reactions *ConversationReactionService
	// @Needs
	attachments *ConversationAttachmentService
	// @Needs
	deliveries *ConversationDeliveryService
}

func (service *ConversationViewService) Views(messages []Message, context ConversationViewContext) []MessageView {
	ordered := service.oldestFirst(messages)
	views := service.viewsOf(ordered, context)
	service.placeInTimeline(views, ordered, context.UnreadCount)
	return views
}

func (service *ConversationViewService) oldestFirst(messages []Message) []Message {
	ordered := slices.Clone(messages)
	slices.Reverse(ordered)
	return ordered
}

func (service *ConversationViewService) viewsOf(messages []Message, context ConversationViewContext) []MessageView {
	views := []MessageView{}
	for _, message := range messages {
		views = append(views, service.view(message, context))
	}
	return views
}

func (service *ConversationViewService) placeInTimeline(views []MessageView, messages []Message, unreadCount int) {
	previousDay := ""
	previous := Message{}
	for index := range views {
		views[index].StartsDay = views[index].DayLabel != previousDay
		views[index].Grouped = service.grouped(previous, messages[index], views[index].StartsDay)
		views[index].NewDivider = index == len(views)-unreadCount
		previousDay = views[index].DayLabel
		previous = messages[index]
	}
}

func (service *ConversationViewService) grouped(previous Message, current Message, startsDay bool) bool {
	return service.allTrue(
		!startsDay,
		strings.EqualFold(previous.Author.Address, current.Author.Address),
		service.within(previous, current),
		previous.Email == nil,
		current.Email == nil,
		previous.DeletedAt == "",
	)
}

func (service *ConversationViewService) within(previous Message, current Message) bool {
	return service.moment(current.CreatedAt).Sub(service.moment(previous.CreatedAt)) <= service.groupWindow()
}

func (service *ConversationViewService) groupWindow() time.Duration {
	return 5 * time.Minute
}

func (service *ConversationViewService) moment(timestamp string) time.Time {
	moment, _ := time.Parse(time.RFC3339, timestamp)
	return moment
}

func (service *ConversationViewService) view(message Message, context ConversationViewContext) MessageView {
	view := service.basics(message, context)
	visible := service.visible(message)
	service.addBody(&view, message, visible, context)
	service.addExtras(&view, message, visible)
	service.addActions(&view, message)
	return view
}

func (service *ConversationViewService) basics(message Message, context ConversationViewContext) MessageView {
	name := cmp.Or(message.Author.DisplayName, message.Author.Address)
	return MessageView{
		MessageId:      message.MessageId,
		AuthorName:     name,
		AuthorInitials: service.initials(name),
		AuthorAddress:  message.Author.Address,
		Outside:        message.Author.Kind != service.memberKind(),
		Mine:           strings.EqualFold(message.Author.Address, context.MyAddress),
		Time:           service.times.Time(message.CreatedAt, context.TimeZone),
		DateTime:       message.CreatedAt,
		DayLabel:       service.times.Day(message.CreatedAt, context.NowMs, context.TimeZone),
		Edited:         message.EditedAt != "",
		Deleted:        message.DeletedAt != "",
		Truncated:      message.Truncated,
		ReplyLabel:     service.countLabel(message.ReplyCount, "reply", "replies"),
		Editing:        message.MessageId == context.EditingId,
		InternalOnly:   service.allTrue(context.Section == service.emailSection(), message.Email == nil),
	}
}

func (service *ConversationViewService) memberKind() string {
	return "member"
}

func (service *ConversationViewService) emailSection() string {
	return "email"
}

func (service *ConversationViewService) addBody(view *MessageView, message Message, visible Message, context ConversationViewContext) {
	start := service.pickNumber(message.Email != nil, service.trailingQuoteStart(visible.Blocks), len(visible.Blocks))
	view.Blocks = service.filledBlocks(visible.Blocks[:start])
	view.QuotedBlocks = service.filledBlocks(visible.Blocks[start:])
	view.ShowQuoted = slices.Contains(context.ShowQuoted, message.MessageId)
	view.MentionsMe = service.mentions(visible.Blocks, context.MyAddress)
	view.Email = service.emailView(message, visible.Blocks, len(view.QuotedBlocks) > 0, context.AllowedSenders)
}

func (service *ConversationViewService) filledBlocks(blocks []Block) []Block {
	filled := []Block{}
	for _, block := range blocks {
		filled = append(filled, service.filledBlock(block))
	}
	return filled
}

func (service *ConversationViewService) filledBlock(block Block) Block {
	block.Spans = append([]Span{}, block.Spans...)
	block.Cells = append([]string{}, block.Cells...)
	return block
}

func (service *ConversationViewService) addExtras(view *MessageView, message Message, visible Message) {
	view.Reactions = service.reactions.Reactions(visible.Reactions, visible.MyReactions)
	view.Attachments = service.attachments.Cards(visible.Attachments)
	view.Deliveries = service.deliveries.Lines(message.Deliveries)
	view.DeliverySummary = service.deliveries.Summary(view.Deliveries)
	view.ShowDelivery = service.allTrue(view.Mine, len(view.Deliveries) > 0)
}

func (service *ConversationViewService) addActions(view *MessageView, message Message) {
	view.CanEdit = service.allTrue(view.Mine, !view.Deleted, message.Email == nil)
	view.CanDelete = service.allTrue(view.Mine, !view.Deleted)
	view.ActionsLabel = "Actions for the message from " + view.AuthorName + " at " + view.Time
}

func (service *ConversationViewService) visible(message Message) Message {
	if message.DeletedAt == "" {
		return message
	}
	return Message{}
}

func (service *ConversationViewService) initials(name string) string {
	letters := []rune(service.firstLetters(strings.Fields(name)) + service.afterFirstLetter(name) + "  ")
	return strings.ToUpper(string(letters[:service.initialsLength()]))
}

func (service *ConversationViewService) firstLetters(words []string) string {
	letters := ""
	for _, word := range words {
		letters += string([]rune(word)[0])
	}
	return letters
}

func (service *ConversationViewService) afterFirstLetter(name string) string {
	letters := []rune(strings.TrimSpace(name) + service.space())
	return string(letters[1:])
}

func (service *ConversationViewService) initialsLength() int {
	return 2
}

func (service *ConversationViewService) space() string {
	return " "
}

func (service *ConversationViewService) trailingQuoteStart(blocks []Block) int {
	start := 0
	for index, block := range blocks {
		start = service.afterNonQuote(start, index, block)
	}
	return start
}

func (service *ConversationViewService) afterNonQuote(start int, index int, block Block) int {
	if block.Kind == "quote" {
		return start
	}
	return index + 1
}

func (service *ConversationViewService) mentions(blocks []Block, myAddress string) bool {
	localPart, _, _ := strings.Cut(strings.ToLower(myAddress), service.atSign())
	text := strings.ToLower(service.plainText(blocks))
	return service.allTrue(localPart != "", strings.Contains(text, service.atSign()+localPart))
}

func (service *ConversationViewService) atSign() string {
	return "@"
}

func (service *ConversationViewService) plainText(blocks []Block) string {
	text := ""
	for _, block := range blocks {
		text += service.blockText(block) + "\n"
	}
	return text
}

func (service *ConversationViewService) blockText(block Block) string {
	text := ""
	for _, span := range block.Spans {
		text += span.Text
	}
	return text
}

func (service *ConversationViewService) emailView(message Message, blocks []Block, hasQuoted bool, allowedSenders []string) MessageEmailView {
	summary := service.summaryOf(message)
	sender := service.senderAddress(summary.From)
	allowed := slices.Contains(service.lowercased(allowedSenders), sender)
	blocked := service.blockedCount(len(service.withoutEmpty(service.remoteUrls(blocks))), allowed)
	check := service.senderCheck(message, summary)
	card := MessageEmailView{
		IsEmail:          true,
		Subject:          summary.Subject,
		From:             summary.From,
		To:               strings.Join(summary.To, service.listSeparator()),
		Cc:               strings.Join(summary.Cc, service.listSeparator()),
		SenderCheck:      check,
		SenderCheckLabel: service.senderLabels()[check],
		SenderCheckInfo:  cmp.Or(service.handoffInfo(message.Author.Address)[check], service.resultsLine(summary.AuthResults)),
		HasQuoted:        hasQuoted,
		BlockedImages:    blocked,
		BlockedLabel:     service.countLabel(blocked, "image hidden", "images hidden"),
		ImagesAllowed:    allowed,
	}
	return service.onlyForEmail(message.Email != nil, card)
}

func (service *ConversationViewService) summaryOf(message Message) MessageEmailSummary {
	if message.Email == nil {
		return MessageEmailSummary{}
	}
	return *message.Email
}

func (service *ConversationViewService) onlyForEmail(isEmail bool, card MessageEmailView) MessageEmailView {
	if isEmail {
		return card
	}
	return MessageEmailView{}
}

func (service *ConversationViewService) listSeparator() string {
	return ", "
}

func (service *ConversationViewService) senderAddress(from string) string {
	start := strings.LastIndex(from, "<") + 1
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(from[start:]), ">"))
}

func (service *ConversationViewService) lowercased(addresses []string) []string {
	lowered := []string{}
	for _, address := range addresses {
		lowered = append(lowered, strings.ToLower(strings.TrimSpace(address)))
	}
	return lowered
}

func (service *ConversationViewService) remoteUrls(blocks []Block) []string {
	urls := []string{}
	for _, block := range blocks {
		urls = append(urls, service.remoteUrlOf(block))
	}
	return urls
}

func (service *ConversationViewService) remoteUrlOf(block Block) string {
	if block.Image == nil {
		return ""
	}
	return block.Image.RemoteUrl
}

func (service *ConversationViewService) withoutEmpty(texts []string) []string {
	return slices.DeleteFunc(texts, service.isEmpty)
}

func (service *ConversationViewService) isEmpty(text string) bool {
	return text == ""
}

func (service *ConversationViewService) blockedCount(count int, allowed bool) int {
	if allowed {
		return 0
	}
	return count
}

func (service *ConversationViewService) senderCheck(message Message, summary MessageEmailSummary) string {
	return cmp.Or(service.handoffWhen(summary), service.verifiedWhen(summary), service.warningCheck())
}

func (service *ConversationViewService) warningCheck() string {
	return "warning"
}

func (service *ConversationViewService) verifiedWhen(summary MessageEmailSummary) string {
	if service.allTrue(strings.EqualFold(summary.AuthResults.DomainPolicy, "pass"), !summary.Suspicious) {
		return service.verifiedCheck()
	}
	return ""
}

func (service *ConversationViewService) verifiedCheck() string {
	return "verified"
}

func (service *ConversationViewService) handoffWhen(summary MessageEmailSummary) string {
	if summary.AuthResults.Handoff {
		return service.handoffCheck()
	}
	return ""
}

func (service *ConversationViewService) handoffCheck() string {
	return "handoff"
}

func (service *ConversationViewService) senderLabels() map[string]string {
	return map[string]string{
		service.verifiedCheck(): "Verified sender",
		service.handoffCheck():  "Sent on this server",
		service.warningCheck():  "Could not verify the sender",
	}
}

func (service *ConversationViewService) handoffInfo(authorAddress string) map[string]string {
	_, domain, _ := strings.Cut(strings.ToLower(authorAddress), service.atSign())
	return map[string]string{service.handoffCheck(): "Sent from " + domain + " on this server"}
}

func (service *ConversationViewService) resultsLine(results MessageAuthResults) string {
	return "SPF " + cmp.Or(results.SenderPolicy, service.missingResult()) +
		" · DKIM " + cmp.Or(results.SignatureCheck, service.missingResult()) +
		" · DMARC " + cmp.Or(results.DomainPolicy, service.missingResult())
}

func (service *ConversationViewService) missingResult() string {
	return "none"
}

func (service *ConversationViewService) countLabel(count int, singular string, plural string) string {
	label, known := map[int]string{0: "", 1: "1 " + singular}[count]
	if known {
		return label
	}
	return strconv.Itoa(count) + service.space() + plural
}

func (service *ConversationViewService) pickNumber(first bool, chosen int, otherwise int) int {
	if first {
		return chosen
	}
	return otherwise
}

func (service *ConversationViewService) allTrue(conditions ...bool) bool {
	return !slices.Contains(conditions, false)
}
