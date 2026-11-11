import (
	"strconv"
	"strings"
)

// @Service
type ComposerWordsService struct{}

func (words *ComposerWordsService) ChipLabel(recipient Recipient) string {
	return words.labels()[words.labelKey(recipient)]
}

func (words *ComposerWordsService) ChipAriaLabel(recipient Recipient) string {
	return recipient.DisplayName + ", " + words.ChipLabel(recipient) + ", " + recipient.Address + ". Open menu"
}

func (words *ComposerWordsService) ChatReason(kind string, address string, domainId string) string {
	reasons := map[string]string{
		words.kindOtherDomain(): "Chat stays inside " + domainId,
		words.kindExternal():    address + " has no Mesail account",
	}
	return reasons[kind]
}

func (words *ComposerWordsService) BothReason(kind string) string {
	reasons := map[string]string{
		words.kindOtherDomain(): "Not possible: this person is in another domain",
		words.kindExternal():    "Not possible without a Mesail account",
	}
	return reasons[kind]
}

func (words *ComposerWordsService) Names(names []string) string {
	return words.joined(names)
}

func (words *ComposerWordsService) Summary(recipients []Recipient) []string {
	lines := []string{}
	lines = words.appendSentence(lines, words.namesWith(recipients, words.deliveryChat()), "a chat message")
	lines = words.appendSentence(lines, words.namesWith(recipients, words.deliveryEmail()), "an email")
	lines = words.appendSentence(lines, words.namesWith(recipients, words.deliveryBoth()), "a chat message and an email")
	return lines
}

func (words *ComposerWordsService) SendLabel(recipients []Recipient) string {
	chats := words.countDeliveries(recipients, words.chatDeliveries())
	emails := words.countDeliveries(recipients, words.emailDeliveries())
	mixed := "Send · " + words.counted(chats, "chat") + ", " + words.counted(emails, "email")
	labels := map[string]string{"chat": "Send", "email": "Send email", "mixed": mixed}
	return labels[words.sendMode(chats, emails)]
}

func (words *ComposerWordsService) EmailMode(recipients []Recipient) bool {
	return words.countDeliveries(recipients, words.emailDeliveries()) > 0
}

func (words *ComposerWordsService) ModeText(recipients []Recipient, domainId string) string {
	chats := words.countDeliveries(recipients, words.chatDeliveries())
	emails := words.countDeliveries(recipients, words.emailDeliveries())
	outside := words.joined(words.namesOutside(recipients))
	emailNames := words.emailNames(recipients)
	texts := map[string]string{
		"chat":  "",
		"email": "This is an email. " + words.outsideSentence(outside, domainId),
		"mixed": "This is a chat and an email. Chat stays inside " + domainId + ", so " + words.joined(emailNames) + " " + words.verb(len(emailNames)) + " an email.",
	}
	return texts[words.sendMode(chats, emails)]
}

func (words *ComposerWordsService) FirstName(displayName string) string {
	first, _, _ := strings.Cut(strings.TrimSpace(displayName), " ")
	return first
}

func (words *ComposerWordsService) Initials(displayName string) string {
	return strings.ToUpper(words.initialsOf(strings.Fields(displayName)))
}

func (words *ComposerWordsService) labelKey(recipient Recipient) string {
	if recipient.State != words.stateReady() {
		return recipient.State
	}
	return recipient.Kind + ":" + recipient.Delivery
}

func (words *ComposerWordsService) labels() map[string]string {
	return map[string]string{
		"checking":           "Checking…",
		"invalid":            "Not valid",
		"member:chat":        "Chat",
		"member:email":       "Email",
		"member:both":        "Chat + email",
		"channel:chat":       "Chat",
		"otherDomain:email":  "Email · other domain",
		"external:email":     "Email",
	}
}

func (words *ComposerWordsService) joined(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

func (words *ComposerWordsService) appendSentence(lines []string, names []string, what string) []string {
	if len(names) == 0 {
		return lines
	}
	return append(lines, words.joined(names)+" "+words.verb(len(names))+" "+what+".")
}

func (words *ComposerWordsService) verb(count int) string {
	if count == 1 {
		return "gets"
	}
	return "get"
}

func (words *ComposerWordsService) namesWith(recipients []Recipient, delivery string) []string {
	names := []string{}
	for _, recipient := range recipients {
		names = words.appendWhen(names, recipient.DisplayName, recipient.Delivery == delivery && recipient.State == words.stateReady())
	}
	return names
}

func (words *ComposerWordsService) emailNames(recipients []Recipient) []string {
	names := []string{}
	for _, recipient := range recipients {
		names = words.appendWhen(names, recipient.DisplayName, words.emailDeliveries()[recipient.Delivery] == 1 && recipient.State == words.stateReady())
	}
	return names
}

func (words *ComposerWordsService) namesOutside(recipients []Recipient) []string {
	names := []string{}
	for _, recipient := range recipients {
		names = words.appendWhen(names, recipient.DisplayName, words.outsideKinds()[recipient.Kind])
	}
	return names
}

func (words *ComposerWordsService) appendWhen(names []string, name string, wanted bool) []string {
	if !wanted {
		return names
	}
	return append(names, name)
}

func (words *ComposerWordsService) outsideSentence(outside string, domainId string) string {
	if outside == "" {
		return "Everyone gets it in their mailbox."
	}
	return outside + " " + words.isOrAre(outside) + " outside " + domainId + "."
}

func (words *ComposerWordsService) isOrAre(names string) string {
	if strings.Contains(names, " and ") {
		return "are"
	}
	return "is"
}

func (words *ComposerWordsService) countDeliveries(recipients []Recipient, counts map[string]int) int {
	total := 0
	for _, recipient := range recipients {
		total += counts[recipient.Delivery]
	}
	return total
}

func (words *ComposerWordsService) sendMode(chats int, emails int) string {
	switch {
	case emails == 0:
		return "chat"
	case chats == 0:
		return "email"
	}
	return "mixed"
}

func (words *ComposerWordsService) counted(count int, noun string) string {
	if count == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(count) + " " + noun + "s"
}

func (words *ComposerWordsService) initialsOf(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return string([]rune(parts[0])[:1])
	}
	return string([]rune(parts[0])[:1]) + string([]rune(parts[len(parts)-1])[:1])
}

func (words *ComposerWordsService) chatDeliveries() map[string]int {
	return map[string]int{words.deliveryChat(): 1, words.deliveryBoth(): 1}
}

func (words *ComposerWordsService) emailDeliveries() map[string]int {
	return map[string]int{words.deliveryEmail(): 1, words.deliveryBoth(): 1}
}

func (words *ComposerWordsService) outsideKinds() map[string]bool {
	return map[string]bool{words.kindOtherDomain(): true, words.kindExternal(): true}
}

func (words *ComposerWordsService) deliveryChat() string {
	return "chat"
}

func (words *ComposerWordsService) deliveryEmail() string {
	return "email"
}

func (words *ComposerWordsService) deliveryBoth() string {
	return "both"
}

func (words *ComposerWordsService) kindOtherDomain() string {
	return "otherDomain"
}

func (words *ComposerWordsService) kindExternal() string {
	return "external"
}

func (words *ComposerWordsService) stateReady() string {
	return "ready"
}
